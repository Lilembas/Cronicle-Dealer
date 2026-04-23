package manager

import (
	"fmt"
	"math"
	"sync"

	"github.com/Knetic/govaluate"

	"github.com/cronicle/cronicle-dealer/internal/models"
)

// FormulaParams 公式参数
type FormulaParams map[string]interface{}

// BuildParamsFromNode 从 Node 资源信息构建公式参数
func BuildParamsFromNode(node *models.Node) FormulaParams {
	return FormulaParams{
		"memory_usage_pct": node.MemoryPercent,
		"memory_usage_abs": node.MemoryUsage,
		"memory_remain_pct": math.Max(0, 100.0-node.MemoryPercent),
		"memory_remain_abs": math.Max(0, node.MemoryTotal-node.MemoryUsage),
		"cpu_usage_pct":    node.CPUUsage,
		"events_used":      float64(node.RunningJobs),
		"events_total":     float64(node.MaxConcurrent),
		"events_remain":    float64(node.MaxConcurrent - node.RunningJobs),
	}
}

// formulaFunctions 公式可用的内置函数
var formulaFunctions = map[string]govaluate.ExpressionFunction{
	"max": func(args ...interface{}) (interface{}, error) {
		if len(args) < 2 {
			return 0, nil
		}
		m := toFloat(args[0])
		for _, a := range args[1:] {
			v := toFloat(a)
			if v > m {
				m = v
			}
		}
		return m, nil
	},
	"min": func(args ...interface{}) (interface{}, error) {
		if len(args) < 2 {
			return 0, nil
		}
		m := toFloat(args[0])
		for _, a := range args[1:] {
			v := toFloat(a)
			if v < m {
				m = v
			}
		}
		return m, nil
	},
	"abs": func(args ...interface{}) (interface{}, error) {
		if len(args) < 1 {
			return 0, nil
		}
		return math.Abs(toFloat(args[0])), nil
	},
	"sqrt": func(args ...interface{}) (interface{}, error) {
		if len(args) < 1 {
			return 0, nil
		}
		return math.Sqrt(toFloat(args[0])), nil
	},
	"pow": func(args ...interface{}) (interface{}, error) {
		if len(args) < 2 {
			return 0, nil
		}
		return math.Pow(toFloat(args[0]), toFloat(args[1])), nil
	},
	"ceil": func(args ...interface{}) (interface{}, error) {
		if len(args) < 1 {
			return 0, nil
		}
		return math.Ceil(toFloat(args[0])), nil
	},
	"floor": func(args ...interface{}) (interface{}, error) {
		if len(args) < 1 {
			return 0, nil
		}
		return math.Floor(toFloat(args[0])), nil
	},
	"round": func(args ...interface{}) (interface{}, error) {
		if len(args) < 1 {
			return 0, nil
		}
		return math.Round(toFloat(args[0])), nil
	},
	"log": func(args ...interface{}) (interface{}, error) {
		if len(args) < 1 {
			return 0, nil
		}
		return math.Log(toFloat(args[0])), nil
	},
}

// expressionCache 公式表达式缓存，避免重复编译
var expressionCache sync.Map

func toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0
	}
}

// getCompiledExpression 获取编译后的表达式（带缓存）
func getCompiledExpression(formula string) (*govaluate.EvaluableExpression, error) {
	if cached, ok := expressionCache.Load(formula); ok {
		return cached.(*govaluate.EvaluableExpression), nil
	}
	expr, err := govaluate.NewEvaluableExpressionWithFunctions(formula, formulaFunctions)
	if err != nil {
		return nil, err
	}
	expressionCache.Store(formula, expr)
	return expr, nil
}

// EvaluateFormula 求值公式
func EvaluateFormula(formula string, params FormulaParams) (float64, error) {
	expr, err := getCompiledExpression(formula)
	if err != nil {
		return 0, fmt.Errorf("invalid formula: %w", err)
	}

	result, err := expr.Evaluate(params)
	if err != nil {
		return 0, fmt.Errorf("evaluation failed: %w", err)
	}

	switch v := result.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("non-numeric result: %T", result)
	}
}

// dummyParams 模拟参数（包级变量，避免重复分配）
var dummyParams = FormulaParams{
	"memory_usage_pct":  50.0,
	"memory_usage_abs":  4.0,
	"memory_remain_pct": 50.0,
	"memory_remain_abs": 4.0,
	"cpu_usage_pct":     30.0,
	"events_used":       3.0,
	"events_total":      10.0,
	"events_remain":     7.0,
}

// ValidateFormula 用模拟参数验证公式语法
func ValidateFormula(formula string) error {
	_, err := EvaluateFormula(formula, dummyParams)
	return err
}

// FormulaParameterInfo 公式参数描述信息（供前端展示）
var FormulaParameterInfo = []struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Unit        string `json:"unit"`
	Description string `json:"description"`
}{
	{"memory_usage_pct", "内存使用百分比", "%", "当前内存使用率"},
	{"memory_usage_abs", "内存使用量", "GB", "当前已用内存绝对值"},
	{"memory_remain_pct", "内存剩余百分比", "%", "剩余内存占比"},
	{"memory_remain_abs", "内存剩余量", "GB", "剩余内存绝对值"},
	{"cpu_usage_pct", "CPU 使用百分比", "%", "CPU 使用率"},
	{"events_used", "已用实例数", "个", "当前运行实例数"},
	{"events_total", "总实例数", "个", "最大并发实例数"},
	{"events_remain", "剩余实例数", "个", "可接受的新实例数"},
}
