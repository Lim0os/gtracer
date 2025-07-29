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
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type Instrumented struct {
	projectPath string
	OutputPath  string
	logger      *slog.Logger
}

func New(projectPath string, outputPath string, logger *slog.Logger) *Instrumented {
	if projectPath == "" {
		fmt.Println("Ошибка: необходимо указать --project")
		os.Exit(1)
	}

	if outputPath == "" {

		outputPath = projectPath + "_instrumented"
	}

	return &Instrumented{projectPath: projectPath, OutputPath: outputPath, logger: logger}
}

func (i *Instrumented) Processed() {

	if err := i.copyProject(); err != nil {
		log.Fatalf("Ошибка копирования проекта: %v", err)
	}

	if err := i.instrumentProject(); err != nil {
		log.Fatalf("Ошибка инструментирования: %v", err)
	}

	if err := updateGoMod(i.OutputPath); err != nil {
		log.Fatalf("Ошибка обновления go.mod: %v", err)
	}

	log.Printf("[GTRACE] Готово!")
}

func (i *Instrumented) copyProject() error {
	ignoreDirs := map[string]struct{}{
		".git":   {},
		"vendor": {},
	}

	return filepath.WalkDir(i.projectPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(i.projectPath, path)
		if err != nil {
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

		dstPath := filepath.Join(i.OutputPath, relPath)
		if d.IsDir() {
			return os.MkdirAll(dstPath, 0o755)
		}

		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
			return err
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}

func (i *Instrumented) instrumentProject() error {
	return filepath.WalkDir(i.OutputPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		return i.instrumentFile(path)
	})
}

func (i *Instrumented) instrumentFile(filePath string) error {
	fset := token.NewFileSet()
	src, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}
	file, err := parser.ParseFile(fset, filePath, src, parser.ParseComments)
	if err != nil {
		return err
	}

	modPath := "gtrace"
	modFile := filepath.Join(i.OutputPath, "go.mod")
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
										rel := relPath(i.OutputPath, filePath)
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
								rel := relPath(i.OutputPath, filePath)
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
				rel := relPath(i.OutputPath, filePath)
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
						rel := relPath(i.OutputPath, filePath)
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
				rel := relPath(i.OutputPath, filePath)
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
			return err
		}
		if err := os.WriteFile(filePath, buf.Bytes(), 0o644); err != nil {
			return err
		}
	}
	return nil
}
