package config

import "os"

var Fingerprinting = env("CNQSO_FINGERPRINTING", "true") == "true"

var TrustedProxies = env("CNQSO_TRUSTED_PROXIES", "127.0.0.1/32,::1/128")

var Port = env("CNQSO_PORT", ":1738")
var UploadDir = env("CNQSO_UPLOAD_DIR", "/app/uploads")
var LibraryDir = env("CNQSO_LIBRARY_DIR", "files")
var CompileTypeScript = env("CNQSO_COMPILE_TYPESCRIPT", "true") == "true"
var TypeScriptCompiler = "tsc" // Pinned in the root package-lock.json.

func env(name, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}
