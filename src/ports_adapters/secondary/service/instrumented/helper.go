package instrumented

import (
	"go/ast"
	"os"
	"path/filepath"
	"strings"
)

func updateGoMod(projectRoot string) error {
	// TODO: реализовать обновление go.mod

	if err := generateGtracePackage(projectRoot); err != nil {
		return err
	}
	return nil
}

func generateGtracePackage(projectRoot string) error {
	dirPath := filepath.Join(projectRoot, "gtrace")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return err
	}
	code := `package gtrace

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"time"
)

func Wrap(fn interface{}, args ...interface{}) []interface{} {
	// Получаем имя функции
	fnName := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	if fnName == "" {
		fnName = "anonymous"
	}

	// Получаем контекст (горутину и caller)
	goroutine := getGoroutineName()
	caller := getCallerInfo(1)
	timestamp := time.Now().UnixNano()

	// Логируем начало вызова
	fmt.Printf("[GTRACE] func_start %s %s %s %d\n",
		goroutine, fnName, caller, timestamp)

	// Вызываем функцию через reflection (как было)
	fnValue := reflect.ValueOf(fn)
	if fnValue.Kind() != reflect.Func {
		panic("Wrap: expected a function")
	}

	fnType := fnValue.Type()
	if len(args) != fnType.NumIn() {
		panic("Wrap: incorrect number of arguments")
	}

	in := make([]reflect.Value, len(args))
	for i, arg := range args {
		argValue := reflect.ValueOf(arg)
		argType := fnType.In(i)
		if !argValue.Type().ConvertibleTo(argType) {
			panic(fmt.Sprintf("Wrap: arg %d (%v) is not convertible to %v", i, argValue.Type(), argType))
		}
		in[i] = argValue.Convert(argType)
	}

	out := fnValue.Call(in)
	results := make([]interface{}, len(out))
	for i, val := range out {
		results[i] = val.Interface()
	}

	// Логируем завершение вызова
	timestampEnd := time.Now().UnixNano()
	fmt.Printf("[GTRACE] func_end %s %s %s %d\n",
		goroutine, fnName, caller, timestampEnd)

	return results
}

// getCallerInfo возвращает информацию о вызывающем коде в формате "файл:строка"
func getCallerInfo(skip int) string {
	_, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return "unknown:0"
	}
	return fmt.Sprintf("%s:%d", file, line)
}

func getGoroutineName() string {
	// Создаем буфер достаточного размера для стека
	buf := make([]byte, 64)
	// Получаем стек текущей горутины
	n := runtime.Stack(buf, false)
	// Берем только первую строку (которая содержит ID горутины)
	stackInfo := string(buf[:n])
	// Формат строки: "goroutine X [status]:"
	lines := strings.SplitN(stackInfo, "\n", 2)
	if len(lines) < 1 {
		return "unknown"
	}
	// Извлекаем "goroutine X" из строки
	fields := strings.Fields(lines[0])
	if len(fields) < 2 {
		return "unknown"
	}
	return fields[1] // возвращаем только номер (X)
}

// WrappedMakeChan логирует создание канала (формат: [GTRACE] channel_create <канал> <файл:строка> <timestamp> <размер_буфера>)
func WrappedMakeChan[T any](name string, ch chan T) chan T {
	caller := getCallerInfo(1)
	buffer := cap(ch)
	timestamp := time.Now().UnixNano()

	fmt.Printf("[GTRACE] channel_create %s %s %d %d\n",
		name, caller, timestamp, buffer)

	return ch
}

// WrappedSend логирует отправку в канал (формат: [GTRACE] channel_send <контекст> <канал> <файл:строка> <timestamp>)
func WrappedSend[T any](ch chan<- T, val T, name string) {
	caller := getCallerInfo(1)
	goroutine := getGoroutineName()
	timestamp := time.Now().UnixNano()

	fmt.Printf("[GTRACE] channel_send %s %s %s %d\n",
		goroutine, name, caller, timestamp)

	ch <- val
}

// WrappedReceive логирует получение из канала (формат: [GTRACE] channel_receive <контекст> <канал> <файл:строка> <timestamp>)
func WrappedReceive[T any](ch <-chan T, name string) T {
	caller := getCallerInfo(1)
	goroutine := getGoroutineName()
	timestamp := time.Now().UnixNano()

	fmt.Printf("[GTRACE] channel_receive %s %s %s %d\n",
		goroutine, name, caller, timestamp)

	return <-ch
}

// WrappedClose логирует закрытие канала (формат: [GTRACE] channel_close <контекст> <канал> <файл:строка> <timestamp>)
func WrappedClose[T any](ch chan<- T, name string) {
	caller := getCallerInfo(1)
	goroutine := getGoroutineName()
	timestamp := time.Now().UnixNano()

	fmt.Printf("[GTRACE] channel_close %s %s %s %d\n",
		goroutine, name, caller, timestamp)

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("[GTRACE] channel_close_error %s %s %s %d %v\n",
				goroutine, name, caller, timestamp, r)
		}
	}()

	close(ch)
}

`
	filePath := filepath.Join(dirPath, "gtrace.go")
	return os.WriteFile(filePath, []byte(code), 0o644)
}

// Вспомогательная функция для относительного пути
func relPath(projectRoot, filePath string) string {
	rel, _ := filepath.Rel(projectRoot, filePath)
	return rel
}

// Вспомогательная функция для получения имени переменной из ast.Expr
func exprToString(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.Ident:
		return v.Name
	default:
		return "" // можно доработать для других случаев
	}
}

func modulePath(modFile []byte) string {
	for _, line := range strings.Split(string(modFile), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}
