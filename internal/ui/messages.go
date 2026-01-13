package ui

import "fmt"

// Success returns a success message with checkmark prefix.
func Success(msg string) string {
	return fmt.Sprintf("%s  %s", StyleSuccess.Render(SymbolSuccess), msg)
}

// SuccessF returns a formatted success message with checkmark prefix.
func SuccessF(format string, args ...any) string {
	return Success(fmt.Sprintf(format, args...))
}

// Info returns an info message with info symbol prefix.
func Info(msg string) string {
	return fmt.Sprintf("%s  %s", StyleInfo.Render(SymbolInfo), msg)
}

// InfoF returns a formatted info message with info symbol prefix.
func InfoF(format string, args ...any) string {
	return Info(fmt.Sprintf(format, args...))
}

// Warning returns a warning message with warning symbol prefix.
func Warning(msg string) string {
	return fmt.Sprintf("%s  %s", StyleWarning.Render(SymbolWarning), msg)
}

// WarningF returns a formatted warning message with warning symbol prefix.
func WarningF(format string, args ...any) string {
	return Warning(fmt.Sprintf(format, args...))
}

// Error returns an error message with X symbol prefix.
func Error(msg string) string {
	return fmt.Sprintf("%s  %s", StyleError.Render(SymbolError), msg)
}

// ErrorF returns a formatted error message with X symbol prefix.
func ErrorF(format string, args ...any) string {
	return Error(fmt.Sprintf(format, args...))
}

// Muted returns text in muted (gray) style.
func Muted(msg string) string {
	return StyleMuted.Render(msg)
}

// Accent returns text in accent (cyan) style for highlighting.
func Accent(msg string) string {
	return StyleAccent.Render(msg)
}

// Bullet returns an indented bullet point.
func Bullet(msg string) string {
	return fmt.Sprintf("  %s  %s", StyleMuted.Render(SymbolBullet), msg)
}

// MutedBullet returns an indented bullet point with muted text.
func MutedBullet(msg string) string {
	return fmt.Sprintf("  %s  %s", StyleMuted.Render(SymbolBullet), StyleMuted.Render(msg))
}
