// Package version — единая строка версии для UI, логов и релизов.
package version

// String — SemVer. Менять по правилу .cursor/rules/localguard-versioning.mdc
const String = "0.3.2"

// Author — отображается в UI рядом с версией и в метаданных сборки.
const Author = "bambutcha"

// AuthorURL — ссылка на профиль / репозиторий.
const AuthorURL = "https://github.com/bambutcha"

// Label — для UI («v0.3.2»).
func Label() string {
	return "v" + String
}

// Credit — «v0.3.2 · bambutcha».
func Credit() string {
	return Label() + " · " + Author
}
