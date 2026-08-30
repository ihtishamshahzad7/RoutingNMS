package olt

import (
	"fmt"
	"strings"
)

type OIDTemplate struct { Base string; IndexOrder []string }
func (t OIDTemplate) Valid() bool { return strings.TrimSpace(t.Base) != "" }
func (t OIDTemplate) Indexed(index ...string) (string,error) { if !t.Valid(){return "",fmt.Errorf("OID template base is empty")}; if len(t.IndexOrder)!=len(index){return "",fmt.Errorf("OID template expects %d indexes, got %d",len(t.IndexOrder),len(index))}; base:=strings.TrimRight(strings.TrimSpace(t.Base),"."); for _,v:=range index{if strings.TrimSpace(v)==""{return "",fmt.Errorf("OID index cannot be empty")};base+="."+strings.TrimSpace(v)}; return base,nil }

type ONUIndexSpec struct { PONPositions []int; ONUPositions []int; Separator string }
func (s ONUIndexSpec) Valid() bool { return len(s.PONPositions)>0 && len(s.ONUPositions)>0 }
func (s ONUIndexSpec) Extract(index string)(string,string,error){ if !s.Valid(){return "","",fmt.Errorf("ONU index specification is not configured")}; parts:=strings.Split(strings.Trim(index,"."),"."); sep:=s.Separator;if sep==""{sep="."};pick:=func(pos []int)(string,error){out:=make([]string,0,len(pos));for _,p:=range pos{if p<0||p>=len(parts){return "",fmt.Errorf("ONU index position %d is outside index %q",p,index)};out=append(out,parts[p])};return strings.Join(out,sep),nil};pon,err:=pick(s.PONPositions);if err!=nil{return "","",err};onu,err:=pick(s.ONUPositions);if err!=nil{return "","",err};return pon,onu,nil }

type VendorProfile struct { Name string; Vendor string; Models []string; PON OIDTemplate; ONU OIDTemplate; IndexSpec ONUIndexSpec; Mapping OIDMapping }
func (p VendorProfile) normalizedMapping() OIDMapping { m:=p.Mapping;if strings.TrimSpace(m.PONName)==""&&p.PON.Valid(){m.PONName=strings.TrimRight(p.PON.Base,".")};if strings.TrimSpace(m.ONUSerial)==""&&p.ONU.Valid(){m.ONUSerial=strings.TrimRight(p.ONU.Base,".")};if !m.ONUIndex.Valid(){m.ONUIndex=p.IndexSpec};return m }
func(p VendorProfile)Valid()bool{return p.normalizedMapping().Valid()}
type ProfileRegistry struct{profiles []VendorProfile}
func NewProfileRegistry(profiles ...VendorProfile)*ProfileRegistry{return &ProfileRegistry{profiles:profiles}}
func(r *ProfileRegistry)Match(vendor,model string)(VendorProfile,bool){vendor,model=strings.ToLower(strings.TrimSpace(vendor)),strings.ToLower(strings.TrimSpace(model));for _,p:=range r.profiles{if strings.ToLower(strings.TrimSpace(p.Vendor))!=vendor{continue};if len(p.Models)==0{p.Mapping=p.normalizedMapping();if !p.Valid(){continue};return p,true};for _,m:=range p.Models{if strings.EqualFold(strings.TrimSpace(m),model){p.Mapping=p.normalizedMapping();if !p.Valid(){continue};return p,true}}};return VendorProfile{},false}
var DefaultProfiles=[]VendorProfile{{Name:"ZTE SNMP",Vendor:"zte"},{Name:"Huawei SNMP",Vendor:"huawei"},{Name:"FiberHome SNMP",Vendor:"fiberhome"}}
func DefaultProfileRegistry()*ProfileRegistry{return NewProfileRegistry(DefaultProfiles...)}
func(r *ProfileRegistry)Resolve(vendor,model string)(OIDMapping,bool){p,ok:=r.Match(vendor,model);if !ok{return OIDMapping{},false};return p.Mapping,true}
func(r *ProfileRegistry)ResolveProfile(vendor,model string)(VendorProfile,bool){return r.Match(vendor,model)}
