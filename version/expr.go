package version

import (
	"fmt"
	"strings"
)

// Env represents a Python environment.
type Env interface {
	Get(k string) (string, error)
}

// Expr represents an expression that can be evaluated given an environment.
type Expr interface {
	Evaluate(env Env) (bool, error)
}

// Evaluate returns true if the dependency should be installed in the given environment.
func (d *Dependency) Evaluate(env Env) (bool, error) {
	matchingExtra := true
	if len(d.Extras) > 0 {
		matchingExtra = false
		for _, e := range d.Extras {
			if e2, _ := env.Get("extra"); e == e2 {
				matchingExtra = true
				break
			}
		}
	}
	if !matchingExtra {
		return false, nil
	}

	// If multiple environment markers are provided all of them must evaluate to true.
	// This logic should be verified as it is not explicitly mentioned in PEP 508.
	for _, sub := range d.expr {
		r, err := sub.Evaluate(env)
		if err != nil {
			return false, err
		}
		if !r {
			return false, nil
		}
	}

	return true, nil
}

type exprMarker struct {
	left, right marker
	op          string
}

func (e exprMarker) Evaluate(env Env) (bool, error) {
	// fmt.Printf("%s %s %s\n", e.left.value, e.op, e.right.value)
	left := e.left.value
	if e.left.env {
		var err error
		left, err = env.Get(left)
		if err != nil {
			return false, err
		}
	}

	right := e.right.value
	if e.right.env {
		var err error
		right, err = env.Get(right)
		if err != nil {
			return false, err
		}
	}

	// Use PEP 440 version comparison operators if both sides are valid versions
	// and the operator is defined in the set of comparison operators for versions.
	if e.op == "in" {
		return strings.Contains(left, right), nil
	} else if e.op == "not in" {
		return !strings.Contains(left, right), nil
	} else if e.op == TripleEqual {
		// '===' is a fallback operator for invalid versions. While technically a
		// version comparison operator it must be used at this point as the versions
		// are unlikely to parse correctly.
		return left == right, nil
	}

	leftVersion, leftValidVersion := Parse(left)
	rightVersion, rightValidVersion := Parse(right)

	if leftValidVersion && rightValidVersion {
		switch e.op {
		case LessOrEqual:
			return Compare(leftVersion, rightVersion) <= 0, nil
		case Less:
			return Compare(leftVersion, rightVersion) < 0, nil
		case NotEqual:
			return Compare(leftVersion, rightVersion) != 0, nil
		case Equal:
			return Compare(leftVersion, rightVersion) == 0, nil
		case GreaterOrEqual:
			return Compare(leftVersion, rightVersion) >= 0, nil
		case Greater:
			return Compare(leftVersion, rightVersion) > 0, nil
		case CompatibleEqual:
			// TODO: Implement support for ~=
			return false, nil
		default:
			return false, fmt.Errorf("unsupported version comparison operator: '%s'", e.op)
		}
	} else {
		switch e.op {
		case LessOrEqual:
			return left <= right, nil
		case Less:
			return left < right, nil
		case NotEqual:
			return left != right, nil
		case Equal:
			return left == right, nil
		case GreaterOrEqual:
			return left >= right, nil
		case Greater:
			return left > right, nil
		case CompatibleEqual:
			return false, fmt.Errorf("'~=' only supported for valid versions")
		default:
			return false, fmt.Errorf("unsupported string comparison operator: '%s'", e.op)
		}
	}
}

type exprOr struct {
	left, right Expr
}

func (e exprOr) Evaluate(env Env) (bool, error) {
	left, err := e.left.Evaluate(env)
	if err != nil {
		return false, err
	}

	right, err := e.right.Evaluate(env)
	if err != nil {
		return false, err
	}

	return left || right, nil
}

type exprAnd struct {
	left, right Expr
}

func (e exprAnd) Evaluate(env Env) (bool, error) {
	left, err := e.left.Evaluate(env)
	if err != nil {
		return false, err
	} else if !left {
		// short-circuit
		return false, nil
	}

	right, err := e.right.Evaluate(env)
	if err != nil {
		return false, err
	}

	return right, nil
}
