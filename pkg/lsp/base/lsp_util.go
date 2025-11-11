package base

import "strings"

func MatchLastSegment(name, nameInFile string, delim string) bool {
	nameLastSegment := LastSegment(name)
	nameInFileLastSegment := LastSegment(nameInFile)

	return nameLastSegment == nameInFileLastSegment
}

func LastSegment(name string) string {
	// Split by delimiter and return last segment
	nameParts := strings.Split(name, ".")
	return nameParts[len(nameParts)-1]
}

func MatchExact(name, nameInFile string) bool {
	return name == nameInFile
}

func MatchIgnoreCase(name, nameInFile string) bool {
	return strings.EqualFold(name, nameInFile)
}

func MatchIgnoreCaseLastSegment(name, nameInFile string, delim string) bool {
	// Split by delimiter and match last segment ignoring case
	nameParts := strings.Split(name, delim)
	nameInFileParts := strings.Split(nameInFile, delim)

	return strings.EqualFold(nameParts[len(nameParts)-1], nameInFileParts[len(nameInFileParts)-1])
}

func RangeInRange(outer, inner Range) bool {
	if inner.Start.Line < outer.Start.Line || inner.End.Line > outer.End.Line {
		return false
	}

	if inner.Start.Line == outer.Start.Line && inner.Start.Character < outer.Start.Character {
		return false
	}
	if inner.End.Line == outer.End.Line && inner.End.Character > outer.End.Character {
		return false
	}
	return true
}
