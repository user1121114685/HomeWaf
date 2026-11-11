//go:build !windows

package main

func holdSystemAwake() (func(), error) { return func() {}, nil }
