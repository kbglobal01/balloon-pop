package main

import "syscall"

// 한글이 깨지지 않도록 콘솔을 UTF-8로 바꿉니다.
func setupConsole() {
	k := syscall.NewLazyDLL("kernel32.dll")
	k.NewProc("SetConsoleOutputCP").Call(65001)
	k.NewProc("SetConsoleCP").Call(65001)
}
