package olt

import (
    "encoding/json"
    "net/http"
    "strconv"
    "strings"
    "github.com/jackc/pgx/v5/pgxpool"
)

type AlertAPI struct { DB *pgxpool.Pool }

func (a AlertAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet { http.Error(w,"method not allowed",http.StatusMethodNotAllowed); return }
    if a.DB == nil { http.Error(w,"database is not initialized",http.StatusServiceUnavailable); return }
    oltID := strings.Trim(strings.TrimPrefix(r.URL.Path,"/olts/"),"/")
    if oltID == "" { http.Error(w,"OLT id required",http.StatusBadRequest); return }
    limit:=50
    if n,err:=strconv.Atoi(r.URL.Query().Get("limit"));err==nil && n>0 && n<=200 { limit=n }
    rows,err:=a.DB.Query(r.Context(),`SELECT id,olt_id,COALESCE(pon_id,''),COALESCE(onu_id,''),code,severity,message,value,status,first_seen,last_seen,cleared_at FROM olt_alerts WHERE olt_id=$1 ORDER BY last_seen DESC LIMIT $2`,oltID,limit)
    if err!=nil { http.Error(w,err.Error(),500); return }; defer rows.Close()
    out:=make([]AlertRecord,0)
    for rows.Next(){var x AlertRecord;var cleared interface{};if err:=rows.Scan(&x.ID,&x.OLTID,&x.PONID,&x.ONUID,&x.Code,&x.Severity,&x.Message,&x.Value,&x.Status,&x.FirstSeen,&x.LastSeen,&cleared);err!=nil{http.Error(w,err.Error(),500);return};out=append(out,x)}
    if err:=rows.Err();err!=nil{http.Error(w,err.Error(),500);return}
    w.Header().Set("Content-Type","application/json"); _=json.NewEncoder(w).Encode(out)
}
