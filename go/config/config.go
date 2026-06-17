package config

import "os"

var Port = env("CNQSO_PORT", ":1738")
var UploadDir = env("CNQSO_UPLOAD_DIR", "/app/uploads")
var CompileTypeScript = env("CNQSO_COMPILE_TYPESCRIPT", "true") == "true"
var TypeScriptCompiler = "tsgo" // "tsgo" is technically in preview. "tsc" works but is slow.

func env(name, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}
