package olt

import (
	"fmt"
	"strconv"
	"strings"
)

// ONUIndexSpec describes how the parent PON index is encoded in an ONU table index.
type ONUIndexSpec struct {
	ParentParts int
	Offset      int
}

func (s ONUIndexSpec) Valid() bool { return s.ParentParts > 0 && s.Offset >= 0 }

// Extract returns parent index, ONU index and an error when the index is invalid.
func (s ONUIndexSpec) Extract(index string) (string,string,error) {
	if !s.Valid(){ return "","",fmt.Errorf("invalid ONU index specification") }
	parts:=strings.Split(strings.Trim(index,"."),".")
	if len(parts)<s.Offset+s.ParentParts+1{return "","",fmt.Errorf("ONU index %q is too short",index)}
	start:=s.Offset; end:=start+s.ParentParts
	parent:=strings.Join(parts[start:end],".")
	onu:=strings.Join(parts[end:],".")
	if _,err:=strconv.Atoi(parts[len(parts)-1]);err!=nil{return "","",fmt.Errorf("ONU index %q has invalid terminal index",index)}
	return parent,onu,nil
}
