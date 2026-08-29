package olt

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// Poller runs a complete OLT discovery/collection cycle and persists results.
type Poller struct {
	Adapter Adapter
	OLT OLT
	Interval time.Duration
	Repo Repository
	OnResult func(PollResult)
	OnAlerts func(OLT, []OpticalAlert)
	OpticalPolicy OpticalPolicy
}

func (p Poller) Run(ctx context.Context) error {
	if p.Adapter == nil { return fmt.Errorf("OLT adapter is required") }
	interval := p.Interval
	if interval < 30*time.Second { interval = 60*time.Second }
	if err := p.poll(ctx); err != nil && ctx.Err() == nil { log.Printf("OLT initial poll failed: %v", err) }
	ticker := time.NewTicker(interval); defer ticker.Stop()
	for { select {
	case <-ctx.Done(): return ctx.Err()
	case <-ticker.C:
		if err:=p.poll(ctx);err!=nil&&ctx.Err()==nil{log.Printf("OLT poll failed: %v",err)}
	} }
}

func (p Poller) poll(ctx context.Context) error {
	ports,err:=p.Adapter.Discover(ctx,p.OLT);if err!=nil{return fmt.Errorf("discover PONs: %w",err)}
	result:=PollResult{PONs:make([]PONPort,0,len(ports)),ONUs:make([]ONU,0),PolledAt:time.Now().UTC()}
	policy:=p.OpticalPolicy;if policy.MinRXDBm==0&&policy.MaxRXDBm==0&&policy.MaxTXDBm==0{policy=DefaultOpticalPolicy()}
	alerts:=make([]OpticalAlert,0)
	for _,port:=range ports{
		onus,err:=p.Adapter.DiscoverONUs(ctx,p.OLT,port);if err!=nil{return fmt.Errorf("discover ONUs on %s: %w",port.Name,err)}
		polled:=make([]ONU,0,len(onus));for _,onu:=range onus{updated,err:=p.Adapter.PollONU(ctx,p.OLT,onu);if err!=nil{if ctx.Err()!=nil{return ctx.Err()};log.Printf("ONU poll failed olt=%s onu=%s: %v",p.OLT.ID,onu.ID,err);polled=append(polled,onu);continue};polled=append(polled,updated);alerts=append(alerts,EvaluateONU(updated,policy)...)}
		port.ONUs=polled;port.ONUCount=len(polled);result.PONs=append(result.PONs,port);result.ONUs=append(result.ONUs,polled...)
	}
	if p.Repo.DB!=nil {if err:=p.Repo.SavePollResult(ctx,p.OLT.ID,result);err!=nil{return fmt.Errorf("persist poll result: %w",err)}}
	if p.OnAlerts!=nil&&len(alerts)>0{p.OnAlerts(p.OLT,alerts)}
	if p.OnResult!=nil{p.OnResult(result)};return nil
}

func NormalizeVendor(v string) string { return strings.ToLower(strings.TrimSpace(v)) }
