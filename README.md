# pinger
Pinger pings hosts and stores results in a database

---
# Database

Hosts are stored in a table along with parameters such as timeout, data size, and protocol (UDP, TCP, ICMP, UDP6, TCP6, ICMP6).

Results are stored in a table, linked to the PK of a host. Stored results include responding address, response code, and whether the data received matches the data sent.
