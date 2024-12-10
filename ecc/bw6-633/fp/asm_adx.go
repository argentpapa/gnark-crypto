//go:build !noadx

// Copyright 2020-2024 Consensys Software Inc.
// Licensed under the Apache License, Version 2.0. See the LICENSE file for details.

// Code generated by consensys/gnark-crypto DO NOT EDIT

package fp

import "golang.org/x/sys/cpu"

var (
	supportAdx = cpu.X86.HasADX && cpu.X86.HasBMI2
	_          = supportAdx
)