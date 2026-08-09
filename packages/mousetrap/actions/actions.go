// Package actions holds one HTTP handler per endpoint. Each is a constructor that
// takes the shared *app.App and returns an http.HandlerFunc, so dependencies are
// injected and the package never imports server (no import cycle).
package actions
