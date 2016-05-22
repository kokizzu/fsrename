package workers

/*
PrefixFilter is actually a regexp filter that generates the pattern from the prefix
*/
func PrefixFilter(prefix string) *RegExpFilter {
	return NewRegExpFilter("^" + prefix)
}
