package accum

import "sort"

type (
	MetadataPieceSectionRefs []MetadataPieceSectionRef
	MetadataPieceSectionRef  struct {
		Offset uint64
		Length uint64
	}
)

// SortAscendingByOffset sorts the MetadataPieceSectionRefs by Offset in ascending order.
func (refs MetadataPieceSectionRefs) SortAscendingByOffset() {
	sort.Slice(refs, func(i, j int) bool {
		return refs[i].Offset < refs[j].Offset
	})
}
