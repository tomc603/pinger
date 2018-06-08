# pinger
Pinger pings hosts and stores results in a database

---
# Future Plans
Add a timer and supporting functions to the Destinations struct, which will dispatch
probes to the probe function(s) through channels. This will mean each Destination effectively
runs a goroutine which will need to be tracked, started, and stopped. Destinations will also
need their timers modified whenever the Destination definition is modified in the database.

Add a polling routine that looks for new, updated, and removed Destinations in the database,
which will need to trigger the addition of new Destination structures, updates to existing ones,
and the stopping and removal of any deleted entries. Each routine can probably watch for updates
to its own entry, but it would probably be easier to utilize a central DB scanning routine.

---
# Database

## Sources
A source is a host running `pinger`. Each host receives its own ID in the **sources** table, which is used in part to
create an identifier value in packets sent to a **destination**.

id | location | host | sourceid | address
--- | --- | --- | --- | --- 
1 | 37 | 12 | 22 | 1.2.3.4
2 | 9 | 2 | 3 | 2.3.4.5

## Destinations
Destinations are addresses stored in a table along with the parameters timeout, ttl/hlim, data size, 
and protocol (UDP, TCP, ICMP, UDP6, TCP6, ICMP6).

id | active | address | protocol | interval | timeout | ttl | data
-- | ------ | ------- | -------- | -------- | ------- | --- | ----
1 | 1 | host1.example.com | 2 | 500 | 1000 | 30 | XXXXXXXXXX
2 | 0 | host2.example.net | 1 | 250 | 250 | 8 | YYYYYYYYYY

## Results
A Result is a response to a probe sent to a **destination**. Responses are stored in a table, linked to the PK of a
**destination**, and the PK of a **source**. A Result includes responding address, response type, response code, and
whether the data received matches the data sent.

id | rtime | address | rsite | rhost | rtt | rtype | rcode | rid | rseq | datamatch
--- | ---- | ------- | ----- | ----- | --- | ----- | ----- | --- | ---- | ---------
121 | 1257894000000000000 | 192.0.2.4 | 9 | 11 | 80 | 1 | 0 | 39821 | 1102 | true
212 | 1257894000987000000 | 2001:0DB8:dead:beef:face::4 | 2 | 6 | 182 | 58 | 0 | 1082| 36 | true
