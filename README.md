# pinger
Pinger pings hosts and stores results in a database

---
# Database

## Sources
A source is a host running `pinger`. Each host receives its own ID in the **sources** table, which is used in part to
create an identifier value in packets sent to a **destination**.

id | hostname | source id
--- | --- | ---
1 | source1.example.net | 24
2 | source2.example.net | 28

## Destinations
Destinations are addresses stored in a table along with the parameters timeout, ttl/hlim, data size, 
and protocol (UDP, TCP, ICMP, UDP6, TCP6, ICMP6).

id | hostname | protocol | timeout | ttl | data size
--- | -------- | -------- | ------- | --- | --------
1 | host1.example.com | udp6 | 500 | | 128
2 | host2.example.net | icmp | 250 | 15 | 64

## Results
A Result is a response to a probe sent to a **destination**. Responses are stored in a table, linked to the PK of a
**destination**, and the PK of a **source**. A Result includes responding address, response type, response code, and
whether the data received matches the data sent.

id | source | destination | responder | response type | response code | data match
--- | --- | --- | --- | --- | --- | ---
121 | 2 | 2 | 192.0.2.4 | 0 | 0 | true
212 | 1 | 1 | 2001:0DB8:dead:beef:face::4 | 0 | 0 | true
