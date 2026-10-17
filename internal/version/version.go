// Package version — единая строка версии для UI, логов и релизов.
package version

// String — SemVer. Менять по правилу .cursor/rules/localguard-versioning.mdc
const String = "0.2.0"

// Label — для UI («v0.2.0»).
func Label() string {
	return "v" + String
}
