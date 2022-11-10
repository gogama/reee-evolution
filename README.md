# reee-evolution
Email filter for Evolution


Rough design:
    There are two programs, a daemon (reeed) and a CLI (reee)
    In version 1.x, you have to build the daemon yourself to provide your own rules.
        If there's ever a version 2.x, reeed will know how to parse rules from text. 
    The CLI is generic, everyone uses the same code.
    They communicate over a Unix socket.

    CLI has these jobs:
        1. Parse args.
        2. If in listing or help mode, query CLI for available groups/rules.
        3. If in filter mode, forward selected group/rule (args) and message (stdin) to reeed and return output/exit code from reeed.
        4. Read logs back from daemon and log anything to stderr at reasonable verbosity.

    Daemon has these jobs.
        1. Live a long time.
        2. When queried for list, provide a listing of available rules/groups.
        3. When queried for filter, execute the requeted group or group/rule on the input message.
        4. Keep an LRU of recent emails in memory and look up by hash to avoid repeat parsing.
        5. Log at reasonable verbosity, specified as 
        6. Sample and write results to sqlite3.


    
