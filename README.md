# Arista Go library [![Build Status](https://travis-ci.org/aristanetworks/goarista.svg?branch=master)](https://travis-ci.org/aristanetworks/goarista) [![codecov.io](http://codecov.io/github/aristanetworks/goarista/coverage.svg?branch=master)](http://codecov.io/github/aristanetworks/goarista?branch=master) [![GoDoc](https://godoc.org/github.com/aristanetworks/goarista?status.png)](https://godoc.org/github.com/aristanetworks/goarista)

## areflect

Helper functions to work with the `reflect` package.  Contains
`ForceExport()`, which bypasses the check in `reflect.Value` that
prevents accessing unexported attributes.

## test

This is a [Go](http://golang.org/) library to help in writing unit tests.

## key

Provides a common type used across various Arista projects, named `key.Key`,
which is used to work around the fact that Go can't let one
use a non-hashable type as a key to a `map`, and we sometimes need to use
a `map[string]interface{}` (or something containing one) as a key to maps.
As a result, we frequently use `map[key.Key]interface{}` instead of just
`map[interface{}]interface{}` when we need a generic key-value collection.

## Examples

TBD
