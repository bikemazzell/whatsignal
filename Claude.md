# WhatSignal


## Memories
- Put all constants into their own separate file
- Keep models separate from main code
- Avoid hard coding strings and magic numbers - put them into constants file
- Do not cheat. Create proper tests. Build and test after every major change to code.
- Always think of alternatives and consider them before implementing a change. Pick the best one.
- Avoid comments as much as possible. Only comment when a line or section is very unclear. Remove excess comments as you encounter them
- WAHA API uses `/api/sendText` format with session in JSON payload, not URL path
- Signal CLI polling requires text-based fallback for message mapping when timestamp IDs don't match
- Always accept both HTTP 200 and 201 status codes as success from WAHA API
- Use automatic Signal polling only - manual polling endpoint removed for cleaner architecture
- Privacy-first logging: sensitive information hidden by default, use --verbose flag to show details
- Logging: phone numbers masked as ***1234, message IDs shortened, content hidden as [hidden]
- All timeouts and intervals configurable via config.json - no magic numbers in code
- Configuration-driven: pollIntervalSec, pollTimeoutSec, retry attempts, etc. all in config
