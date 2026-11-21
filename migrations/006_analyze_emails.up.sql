-- Give the planner statistics for the expression index from 005 (an
-- expression index has none until the table is analyzed). ANALYZE takes a
-- lock that does not block inserts.
ANALYZE emails;
