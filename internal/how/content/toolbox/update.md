# Updating a toolbox item

1. Write the new version in your task directory and run it, then `lgrass toolbox update <slug> <file>...`. A file with the same name replaces the old one.
2. `--drop <file>` removes a file from the item. An item keeps at least one file.
3. `--about "<sentence>"` and `--tags <tag,tag>` rewrite its inventory line when what it does has changed.

The biblio history keeps the earlier version, so no copy goes beside it.
