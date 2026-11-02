.DEFAULT_GOAL := all
.NOTPARALLEL:
.PHONY: all main plugins man install install-main install-plugins install-data clean distclean dist tags
all main plugins man install install-main install-plugins install-data dist tags:
	+python3 scripts/build_local.py make $@

clean distclean:
	python3 scripts/build_local.py clean
