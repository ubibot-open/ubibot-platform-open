module github.com/ubibot/ubibot-platform-open

go 1.23

require (
	gorm.io/driver/sqlite v1.6.0
	gorm.io/gorm v1.31.2
)

require (
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/mattn/go-sqlite3 v1.14.22 // indirect
	golang.org/x/text v0.20.0 // indirect
)

// gorm.io/* are vanity import paths that resolve via an HTTP meta-tag
// redirect to these same github.com repos. This environment's module
// fetches can reach github.com directly but not the gorm.io redirect (or
// golang.org, modernc.org, gitlab.com — several otherwise-natural
// dependencies live behind hosts this network can't reach), so the
// replace pins each to its real repo instead of the vanity path.
replace (
	golang.org/x/text => github.com/golang/text v0.20.0
	gorm.io/driver/sqlite => github.com/go-gorm/sqlite v1.6.0
	gorm.io/gorm => github.com/go-gorm/gorm v1.31.2
)
