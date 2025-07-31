package instrumented

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"io/fs"
	"io/ioutil"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type Instrumented struct {
	logger *slog.Logger
}

func New(logger *slog.Logger) *Instrumented {
	return &Instrumented{logger: logger}
}

func (i *Instrumented) Processed(projectPath string, outputPath string) error {
	i.logger.Info("Начало обработки проекта", "projectPath", projectPath, "outputPath", outputPath)

	if err := i.copyProject(projectPath, outputPath); err != nil {
		i.logger.Error("Ошибка копирования проекта", "error", err)
		return fmt.Errorf("ошибка копирования проекта: %v", err)
	}

	if err := i.instrumentProject(outputPath); err != nil {
		i.logger.Error("Ошибка инструментирования", "error", err)
		return fmt.Errorf("ошибка инструментирования: %v", err)
	}

	if err := updateGoMod(outputPath); err != nil {
		i.logger.Error("Ошибка обновления go.mod", "error", err)
		return fmt.Errorf("ошибка обновления go.mod: %v", err)
	}

	i.logger.Info("Проект успешно обработан")
	return nil
}

func (i *Instrumented) copyProject(projectPath string, outputPath string) error {
	i.logger.Info("Начало копирования проекта", "source", projectPath, "destination", outputPath)

	ignoreDirs := map[string]struct{}{
		".git":   {},
		"vendor": {},
	}

	return filepath.WalkDir(projectPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			i.logger.Error("Ошибка при обходе директории", "path", path, "error", err)
			return err
		}

		relPath, err := filepath.Rel(projectPath, path)
		if err != nil {
			i.logger.Error("Ошибка получения относительного пути", "path", path, "error", err)
			return err
		}
		if relPath == "." {
			return nil
		}

		parts := strings.Split(relPath, string(filepath.Separator))
		for _, part := range parts {
			if part == "" {
				continue
			}
			if _, skip := ignoreDirs[part]; skip {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if strings.HasPrefix(part, ".") {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
		}

		if !d.IsDir() && strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}

		dstPath := filepath.Join(outputPath, relPath)
		if d.IsDir() {
			if err := os.MkdirAll(dstPath, 0o755); err != nil {
				i.logger.Error("Ошибка создания директории", "path", dstPath, "error", err)
				return err
			}
			i.logger.Debug("Создана директория", "path", dstPath)
			return nil
		}

		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
			i.logger.Error("Ошибка создания родительской директории", "path", dstPath, "error", err)
			return err
		}

		srcFile, err := os.Open(path)
		if err != nil {
			i.logger.Error("Ошибка открытия исходного файла", "path", path, "error", err)
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.Create(dstPath)
		if err != nil {
			i.logger.Error("Ошибка создания целевого файла", "path", dstPath, "error", err)
			return err
		}
		defer dstFile.Close()

		if _, err := io.Copy(dstFile, srcFile); err != nil {
			i.logger.Error("Ошибка копирования файла", "source", path, "destination", dstPath, "error", err)
			return err
		}
		i.logger.Debug("Файл скопирован", "source", path, "destination", dstPath)
		return nil
	})
}

func (i *Instrumented) instrumentProject(outputPath string) error {
	i.logger.Info("Начало инструментирования проекта", "outputPath", outputPath)

	return filepath.WalkDir(outputPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			i.logger.Error("Ошибка при обходе директории", "path", path, "error", err)
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		i.logger.Debug("Инструментирование файла", "path", path)
		return i.instrumentFile(outputPath, path)
	})
}

func (i *Instrumented) instrumentFile(outputPath, filePath string) error {
	i.logger.Debug("Начало инструментирования файла", "filePath", filePath)

	fset := token.NewFileSet()
	src, err := ioutil.ReadFile(filePath)
	if err != nil {
		i.logger.Error("Ошибка чтения файла", "filePath", filePath, "error", err)
		return err
	}
	file, err := parser.ParseFile(fset, filePath, src, parser.ParseComments)
	if err != nil {
		i.logger.Error("Ошибка парсинга файла", "filePath", filePath, "error", err)
		return err
	}

	modPath := "gtrace"
	modFile := filepath.Join(outputPath, "go.mod")
	if data, err := ioutil.ReadFile(modFile); err == nil {
		if modulePath := modulePath(data); modulePath != "" {
			modPath = modulePath + "/gtrace"
		}
	}

	hasGtrace := false
	expectedImportPath := fmt.Sprintf(`"%s"`, modPath)
	for _, imp := range file.Imports {
		if imp.Path != nil && imp.Path.Value == expectedImportPath {
			hasGtrace = true
			break
		}
	}

	if !hasGtrace {
		newImport := &ast.ImportSpec{
			Path: &ast.BasicLit{
				Kind:  token.STRING,
				Value: expectedImportPath,
			},
		}

		importDecl := &ast.GenDecl{
			Tok:   token.IMPORT,
			Specs: []ast.Spec{newImport},
		}

		file.Decls = append([]ast.Decl{importDecl}, file.Decls...)
	}

	var newDecls []ast.Decl
	modified := false

	var processBlock func(*ast.BlockStmt) *ast.BlockStmt
	processBlock = func(block *ast.BlockStmt) *ast.BlockStmt {
		if block == nil {
			return nil
		}

		var newList []ast.Stmt
		for _, stmt := range block.List {
			switch s := stmt.(type) {
			case *ast.ForStmt:
				s.Body = processBlock(s.Body)
				newList = append(newList, s)
				continue
			case *ast.RangeStmt:
				s.Body = processBlock(s.Body)
				newList = append(newList, s)
				continue
			case *ast.IfStmt:
				s.Body = processBlock(s.Body)
				if s.Else != nil {
					if elseBlock, ok := s.Else.(*ast.BlockStmt); ok {
						s.Else = processBlock(elseBlock)
					} else if elseIf, ok := s.Else.(*ast.IfStmt); ok {
						elseIf.Body = processBlock(elseIf.Body)
						s.Else = elseIf
					}
				}
				newList = append(newList, s)
				continue
			case *ast.SwitchStmt:
				if s.Body != nil {
					for _, caseStmt := range s.Body.List {
						if caseClause, ok := caseStmt.(*ast.CaseClause); ok {
							caseClause.Body = processBlock(&ast.BlockStmt{List: caseClause.Body}).List
						}
					}
				}
				newList = append(newList, s)
				continue
			case *ast.TypeSwitchStmt:
				if s.Body != nil {
					for _, caseStmt := range s.Body.List {
						if caseClause, ok := caseStmt.(*ast.CaseClause); ok {
							caseClause.Body = processBlock(&ast.BlockStmt{List: caseClause.Body}).List
						}
					}
				}
				newList = append(newList, s)
				continue
			case *ast.SelectStmt:
				if s.Body != nil {
					for _, commStmt := range s.Body.List {
						if commClause, ok := commStmt.(*ast.CommClause); ok {
							commClause.Body = processBlock(&ast.BlockStmt{List: commClause.Body}).List
						}
					}
				}
				newList = append(newList, s)
				continue
			case *ast.BlockStmt:
				s = processBlock(s)
				newList = append(newList, s)
				continue
			}

			// 1. make(chan ...) — ищем объявления переменных и присваивания с make(chan ...)
			if declStmt, ok := stmt.(*ast.DeclStmt); ok {
				if genDecl, ok := declStmt.Decl.(*ast.GenDecl); ok && genDecl.Tok == token.VAR {
					for _, spec := range genDecl.Specs {
						vs, ok := spec.(*ast.ValueSpec)
						if !ok {
							continue
						}
						for j, v := range vs.Values {
							call, ok := v.(*ast.CallExpr)
							if ok {
								ident, ok := call.Fun.(*ast.Ident)
								if ok && ident.Name == "make" && len(call.Args) > 0 {
									if _, ok := call.Args[0].(*ast.ChanType); ok && j < len(vs.Names) {
										name := vs.Names[j].Name
										rel := relPath(outputPath, filePath)
										line := fset.Position(v.Pos()).Line
										wrapped := &ast.AssignStmt{
											Lhs: []ast.Expr{ast.NewIdent(name)},
											Tok: token.ASSIGN,
											Rhs: []ast.Expr{
												&ast.CallExpr{
													Fun: ast.NewIdent("gtrace.WrappedMakeChan"),
													Args: []ast.Expr{
														&ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("\"%s:%d\"", rel, line)},
														ast.NewIdent(name),
													},
												},
											},
										}
										newList = append(newList, stmt, wrapped)
										modified = true
										goto nextStmt
									}
								}
							}
						}
					}
				}
			}
			if assignStmt, ok := stmt.(*ast.AssignStmt); ok {
				for j, rhs := range assignStmt.Rhs {
					call, ok := rhs.(*ast.CallExpr)
					if ok {
						ident, ok := call.Fun.(*ast.Ident)
						if ok && ident.Name == "make" && len(call.Args) > 0 {
							if _, ok := call.Args[0].(*ast.ChanType); ok && j < len(assignStmt.Lhs) {
								name := exprToString(assignStmt.Lhs[j])
								rel := relPath(outputPath, filePath)
								line := fset.Position(rhs.Pos()).Line
								wrapped := &ast.AssignStmt{
									Lhs: []ast.Expr{ast.NewIdent(name)},
									Tok: token.ASSIGN,
									Rhs: []ast.Expr{
										&ast.CallExpr{
											Fun: ast.NewIdent("gtrace.WrappedMakeChan"),
											Args: []ast.Expr{
												&ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("\"%s:%d\"", rel, line)},
												ast.NewIdent(name),
											},
										},
									},
								}
								newList = append(newList, stmt, wrapped)
								modified = true
								goto nextStmt
							}
						}
					}
				}
			}

			// 2. go ... — оборачиваем вызовы через gtrace.Wrap
			if goStmt, ok := stmt.(*ast.GoStmt); ok {
				call := goStmt.Call
				if call != nil {
					var fun ast.Expr
					var args []ast.Expr
					fun = call.Fun
					args = call.Args
					wrapCall := &ast.CallExpr{
						Fun:  ast.NewIdent("gtrace.Wrap"),
						Args: append([]ast.Expr{fun}, args...),
					}
					newGo := &ast.GoStmt{Call: wrapCall}
					newList = append(newList, newGo)
					modified = true
					goto nextStmt
				}
			}

			// 3. ch <- val — заменяем на gtrace.WrappedSend
			if send, ok := stmt.(*ast.SendStmt); ok {
				rel := relPath(outputPath, filePath)
				line := fset.Position(send.Pos()).Line
				wrapped := &ast.ExprStmt{
					X: &ast.CallExpr{
						Fun: ast.NewIdent("gtrace.WrappedSend"),
						Args: []ast.Expr{
							send.Chan,
							send.Value,
							&ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("\"%s:%d\"", rel, line)},
						},
					},
				}
				newList = append(newList, wrapped)
				modified = true
				goto nextStmt
			}

			// 4. close(ch) — заменяем на gtrace.WrappedClose
			if exprStmt, ok := stmt.(*ast.ExprStmt); ok {
				if call, ok := exprStmt.X.(*ast.CallExpr); ok {
					if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "close" && len(call.Args) == 1 {
						rel := relPath(outputPath, filePath)
						line := fset.Position(call.Pos()).Line
						wrapped := &ast.ExprStmt{
							X: &ast.CallExpr{
								Fun: ast.NewIdent("gtrace.WrappedClose"),
								Args: []ast.Expr{
									call.Args[0],
									&ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("\"%s:%d\"", rel, line)},
								},
							},
						}
						newList = append(newList, wrapped)
						modified = true
						goto nextStmt
					}
				}
			}

			// 5. for v := range ch — первой строкой тела цикла добавляем gtrace.WrappedReceive
			if forStmt, ok := stmt.(*ast.RangeStmt); ok && forStmt.X != nil && forStmt.Tok == token.ARROW {
				rel := relPath(outputPath, filePath)
				line := fset.Position(forStmt.Pos()).Line
				wrapped := &ast.ExprStmt{
					X: &ast.CallExpr{
						Fun: ast.NewIdent("gtrace.WrappedReceive"),
						Args: []ast.Expr{
							forStmt.X,
							&ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("\"%s:%d\"", rel, line)},
						},
					},
				}
				if forStmt.Body != nil {
					forStmt.Body.List = append([]ast.Stmt{wrapped}, forStmt.Body.List...)
					modified = true
				}
				newList = append(newList, forStmt)
				goto nextStmt
			}

			// По умолчанию — просто добавляем stmt
			newList = append(newList, stmt)
		nextStmt:
		}
		return &ast.BlockStmt{List: newList}
	}

	// Обрабатываем все объявления в файле
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			newDecls = append(newDecls, decl)
			continue
		}
		fn.Body = processBlock(fn.Body)
		newDecls = append(newDecls, fn)
	}
	file.Decls = newDecls

	if modified {
		var buf bytes.Buffer
		if err := printer.Fprint(&buf, fset, file); err != nil {
			i.logger.Error("Ошибка форматирования AST", "filePath", filePath, "error", err)
			return err
		}
		if err := os.WriteFile(filePath, buf.Bytes(), 0o644); err != nil {
			i.logger.Error("Ошибка записи изменённого файла", "filePath", filePath, "error", err)
			return err
		}
		i.logger.Debug("Файл изменён и сохранён", "filePath", filePath)
	}
	i.logger.Debug("Инструментирование файла завершено", "filePath", filePath)
	return nil
}
