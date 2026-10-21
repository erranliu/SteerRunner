package contracts

import "regexp"

// NamePrefix is a caller-owned naming hint, independent of correlation values.
const MaxNamePrefixLength = 48
const NamePrefixPattern = `^[a-z0-9]+(-[a-z0-9]+)*$`

var namePrefixPattern = regexp.MustCompile(NamePrefixPattern)
