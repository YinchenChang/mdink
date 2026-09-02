//go:build !windows

package main

func installDir() string              { return "" }
func isInstalled() bool               { return false }
func runningFromInstallDir() bool     { return false }
func doInstall() error                { return errWindowsOnly() }
func doUninstall() error              { return errWindowsOnly() }
func finishUninstall()                {}
func errWindowsOnly() error           { return errf("安裝與右鍵選單僅支援 Windows") }
func errf(s string) error             { return &simpleErr{s} }

type simpleErr struct{ s string }

func (e *simpleErr) Error() string { return e.s }
