package main

import (
	"fmt"
	"reflect"
	"testing"
)

// runPermutations generates permutations from the given params struct and runs the callback as a
// subtest for each permutation.
//
// For now, the given struct must only contain boolean fields.
func runPermutations[T any](t *testing.T, params T, f func(t *testing.T, params T)) {
	t.Helper()

	v := reflect.ValueOf(params)

	numberOfPermutations := 1
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).Kind() != reflect.Bool {
			t.Fatal("unsupported permutation parameter type: " + v.Field(i).Kind().String())
		}
		numberOfPermutations *= 2
	}

	type run struct {
		description string
		params      T
	}
	var runs []run

	for i := 0; i < numberOfPermutations; i++ {
		var description string
		var params T
		paramsValue := reflect.ValueOf(&params).Elem()

		for j := 0; j < v.NumField(); j++ {
			enabled := (i & (1 << j)) > 0
			if len(description) > 0 {
				description += ","
			}
			description += fmt.Sprintf("%s=%v", v.Type().Field(j).Name, enabled)

			paramsValue.Field(j).SetBool(enabled)
		}

		runs = append(runs, run{description, params})
	}

	for _, r := range runs {
		t.Run(r.description, func(t *testing.T) {
			t.Helper()
			f(t, r.params)
		})
	}
}
