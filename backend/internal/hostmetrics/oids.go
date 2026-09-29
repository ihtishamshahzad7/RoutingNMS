package hostmetrics

// Standard HOST-RESOURCES-MIB (RFC 2790) OIDs, used as the default when a
// device isn't specifically MikroTik or Cisco (see poller.go's vendor
// dispatch). hrProcessorLoad has one row per CPU -- multi-core devices
// report several -- so it's walked and averaged, not GET'd directly.
// hrStorageTable is walked in full and filtered down to the row whose
// hrStorageType equals HrStorageRAMOID (the physical-memory entry);
// hrStorageAllocationUnits isn't needed for a percentage since it cancels
// out of used/size.
const (
	HrProcessorLoadOID = "1.3.6.1.2.1.25.3.3.1.2" // walk column: percent busy (0-100) per CPU
	HrStorageTypeOID    = "1.3.6.1.2.1.25.2.3.1.2" // walk column: an hrStorageTypes OID per row
	HrStorageSizeOID    = "1.3.6.1.2.1.25.2.3.1.5" // walk column: size in allocation units
	HrStorageUsedOID    = "1.3.6.1.2.1.25.2.3.1.6" // walk column: used in allocation units
	HrStorageRAMOID     = "1.3.6.1.2.1.25.2.1.2"   // the hrStorageType value identifying physical RAM
)

// MikroTik RouterOS doesn't implement HOST-RESOURCES-MIB, so its own
// enterprise MIB is used instead. Confirmed against a real device by the
// user (2026-09-29) before this poller was built.
const (
	MikroTikCPUOID      = ".1.3.6.1.4.1.14988.1.1.3.10.0" // mtxrHlCpuLoad: integer percent 0-100
	MikroTikMemTotalOID = ".1.3.6.1.4.1.14988.1.1.3.1.0"  // mtxrHlMemoryTotal: bytes
	MikroTikMemFreeOID  = ".1.3.6.1.4.1.14988.1.1.3.2.0"  // mtxrHlMemoryFree: bytes
)

// Cisco IOS: CISCO-PROCESS-MIB / CISCO-MEMORY-POOL-MIB. Walk-based, not a
// fixed GET, since the row index (CPU core / memory pool) isn't consistent
// across platforms. IOS only -- NX-OS uses different MIBs entirely and is
// deliberately out of scope for this sub-item (see the device detail
// page's Ports/CPU history report). Confirmed against a real device by the
// user (2026-09-29) before this poller was built.
const (
	CiscoCPUTotal5MinRevOID = "1.3.6.1.4.1.9.9.109.1.1.1.1.8" // cpmCPUTotal5minRev: walk column, average
	CiscoMemPoolNameOID     = "1.3.6.1.4.1.9.9.48.1.1.1.2"    // ciscoMemoryPoolName: walk column
	CiscoMemPoolUsedOID     = "1.3.6.1.4.1.9.9.48.1.1.1.5"    // ciscoMemoryPoolUsed: walk column
	CiscoMemPoolFreeOID     = "1.3.6.1.4.1.9.9.48.1.1.1.6"    // ciscoMemoryPoolFree: walk column
)
