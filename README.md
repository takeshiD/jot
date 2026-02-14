# jot
jot is a memo tool.

- one line memo
- quickly
- keep

# Basic Usage
`jot` just only has operation `create`, `remove`, `list`.

## Create memo
```bash
jot
▌ that's all
▌ shopping list: egg, apple ... (14 lines)
▌ ```bash .... (20 lines)
> nix is so good
```

## List memo
```bash
jost list
| ID     | Memo                          | CreateDate          | Create At              |
| ------ | ----------------------------- | ----------          | ---------              |
| b32adh | that's all                    | 2025-01-01 10:23:12 | ~/dotfiles             |
| ga5uha | shopping list: egg, apple ... | 2025-10-11 12:29:26 | ~/ex_prog/ex_rust/jot  |
| kjq91j | ```bash ....                  | 2026-01-09 21:43:36 | ~/ex_prog/ex_py/cxstat |
| 1h3rgo | nix is so good                | 2026-02-11 16:23:16 | /etc/nix/nix.conf      |
```

## Remove memo
```bash
jost remove b32
Removed ID=`b32adh` Memo=that's all... CreatedDate
```

# Advanced Usage

