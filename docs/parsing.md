# command hierarchy

- root
  - command1
    - subcommand1
      - p1 (single positional)
    - subcommand2
  - command2
    - subcommand1
    - subcommand2
      - variadic positionals (vals1...)
      - flag1 w/variadic positionals (parses into a list) (fval1...)

# allowed calls

`targ command2 subcommand2 val1 val2 val3 --flag1 fval1 fval2 fval3 -- command1 subcommand1`
`targ command2 subcommand2  --flag1 fval1 fval2 fval3 -- val1 val2 val3 -- command1 subcommand1`
`targ command1 subcommand1 p1 command2 subcommand2  --flag1 fval1 ` (no -- needed. command1 positionals are required and
discrete. --flag starts flag parsing for command2. flag parsing terminates with end of input.)

# disallowed calls

`targ command2 subcommand2 val1 val2 --flag1 fval1 fval2 fval3 val3 -- command1 subcommand1` (val3 would be another fval
variadic flag positional)
`targ command2 subcommand2 val1 val2 --flag1 fval1 fval2 fval3 -- val3 -- command1 subcommand1` (val3 would be another
command - the flag terminated the positionals)

# rules

- flags are prefixed with `--` or `-`
- commands and flags may have positionals (single or variadic)
- commands may have multiple sets of independent positionals (p1, p2, p3, p4v1, p4v2, p4v3, etc)
- flags may only have a single set of positionals
- positional sets may be interspersed with flags (p1, --flag1 f1, p2)
- positionals within a set may not be interspersed with flags (p4v1, --flag1 f1, p4v2 is invalid)
- positional sets are parsed in the order in which they are declared (p2 cannot come before p1)
- variadic positionals (for commands or for flags) are terminated by any of: the next flag, a `--` token, or the end of input
- additional commands are only parsed after all of the positional sets for the current command have been parsed
