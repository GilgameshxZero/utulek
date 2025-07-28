# GNU make (gmake) & nmake shared makefile.
# Conditional logic implemented with differing end-of-line comment syntax.
# To enable multithreaded compilation, nmake compiles all files together with
# /MP, while gmake compiles separately with -jN.

# Variables:
# PROJ: a repository can have multiple *project*s, each in their own directory
# 	(default = experiment).
# BIN: name of the project; a project may have multiple (default = PROJ).
# SRC: target filenames (can be multiple) to link, starting from the project
# 	directory (default = BIN.cpp).
# BUILD: 0 (default), 1, or 2, specifying the type of build.
# ARCH: 64 (default) or 86, specifying the architecture to build for. The
# 	environment may need to be initialized accordingly (e.g. for cl.exe).
# ARGS: command-line arguments to pass to the executable to be run.
# INCL: set from caller on Windows, enumerates header file directory wildcards.

# Targets:
# build (default): compile & link.
# run: compile, link, and run.
# clean: remove intermediates directory.

# Default target defined before any "fake" targets.
__default: _default

# ------------------------------------------------------------------------------
ifdef MAKEDIR: # gmake: false; nmake: unused target
!ifdef MAKEDIR # gmake: not seen; nmake: true
# ------------------------------------------------------------------------------

ROOT_DIR = ..^\

# nmake specific commands.
REPO_NAME = utulek
!IFNDEF PROJ
PROJ_NAME = experiment
!ELSE
PROJ_NAME = $(PROJ)
!ENDIF
!IFNDEF BIN
BIN_NAME = $(PROJ_NAME)
!ELSE
BIN_NAME = $(BIN)
!ENDIF
!IFNDEF SRC
SRC_NAME = $(BIN_NAME).cpp
!ELSE
SRC_NAME = $(SRC)
!ENDIF
!IFNDEF BUILD
BUILD_NAME = debug
!ELSE IF $(BUILD) == 2
BUILD_NAME = instrument
!ELSE IF $(BUILD) == 1
BUILD_NAME = release
!ELSE
BUILD_NAME = debug
!ENDIF
!IFNDEF ARCH
ARCH = 64
!ENDIF
# Makefile fails if directory doesn't exist.
!IF [IF EXIST $(ROOT_DIR)experiment\$(BIN_NAME).cpp-assets\ EXIT /B 1] == 1
INCL = $(INCL) $(ROOT_DIR)experiment\$(BIN_NAME).cpp-assets\*
!ENDIF

# The directory structure here must be consistent with macro substitution
# structure utilized later.
BIN_DIR = $(ROOT_DIR)bin^\
PROJ_BIN = $(BIN_DIR)$(PROJ_NAME)\$(BIN_NAME).$(BUILD_NAME).x$(ARCH).exe
PROJ_BIN_DIR = $(BIN_DIR)$(PROJ_NAME)^\
OBJ_DIR = $(ROOT_DIR)obj^\
PROJ_OBJ_DIR = $(OBJ_DIR)$(PROJ_NAME)^\
PROJ_SRC_DIR = $(ROOT_DIR)$(PROJ_NAME)^\
PROJ_SRC = $(patsubsti %,$(PROJ_SRC_DIR)\%,$(SRC_NAME))

COMPILER = cl /c
COMPILER_FLAGS_INCLUDES = /I $(ROOT_DIR)rain\include /I $(ROOT_DIR)include\
	/I $(ROOT_DIR)experiment
COMPILER_FLAGS_COMMON = $(COMPILER_FLAGS_INCLUDES) /std:c++20 /D _CONSOLE\
	/Fd$(PROJ_OBJ_DIR)$(BUILD_NAME)\x$(ARCH)\$(BIN_NAME).pdb\
	/Fo$(PROJ_OBJ_DIR)$(BUILD_NAME)\x$(ARCH)\ \
	/Fp$(PROJ_OBJ_DIR)$(BUILD_NAME)\x$(ARCH)\$(BIN_NAME).pch\
	/fp:fast /MP /permissive- /Zc:wchar_t /Zc:forScope /Zc:inline /Zf\
	/GS /W3 /WX- /wd4250 /wd4068 /sdl /diagnostics:column /EHsc /Gm- /nologo
COMPILER_FLAGS_DEBUG = /D _DEBUG /MDd /Od /RTC1 /JMC /ZI
COMPILER_FLAGS_RELEASE = /D NDEBUG /MT /O2 /Oi /GL /Gy /Zi
COMPILER_FLAGS_INSTRUMENT = $(COMPILER_FLAGS_DEBUG) /Fa$(PROJ_OBJ_DIR)
COMPILER_FLAGS_64 =
COMPILER_FLAGS_86 =

LINKER = link
LINKER_FLAGS_LIBRARIES =
LINKER_FLAGS_COMMON = $(LINKER_FLAGS_LIBRARIES) /OUT:$(PROJ_BIN)\
	/PDB:$(PROJ_OBJ_DIR)$(BUILD_NAME)\x$(ARCH)\$(BIN_NAME).pdb\
	/ILK:$(PROJ_OBJ_DIR)$(BUILD_NAME)\x$(ARCH)\$(BIN_NAME).ilk\
	/MANIFESTUAC:"level='asInvoker' uiAccess='false'"\
	/MANIFESTFILE:$(PROJ_OBJ_DIR)$(BUILD_NAME)\x$(ARCH)\$(BIN_NAME).exe.intermediate.manifest\
	/LTCGOUT:$(PROJ_OBJ_DIR)$(BUILD_NAME)\x$(ARCH)\$(BIN_NAME).iobj\
	/SUBSYSTEM:CONSOLE /NOLOGO
LINKER_FLAGS_DEBUG = /DEBUG
LINKER_FLAGS_RELEASE = /INCREMENTAL:NO /OPT:ICF /OPT:REF /LTCG:incremental
LINKER_FLAGS_INSTRUMENT = $(LINKER_FLAGS_DEBUG)
LINKER_FLAGS_64 = /MACHINE:x64
LINKER_FLAGS_86 = /MACHINE:x86

!IF "$(BUILD_NAME)" == "instrument"
COMPILER_FLAGS = $(COMPILER_FLAGS_COMMON) $(COMPILER_FLAGS_INSTRUMENT)
LINKER_FLAGS = $(LINKER_FLAGS_COMMON) $(LINKER_FLAGS_INSTRUMENT)
!ELSE IF "$(BUILD_NAME)" == "release"
COMPILER_FLAGS = $(COMPILER_FLAGS_COMMON) $(COMPILER_FLAGS_RELEASE)
LINKER_FLAGS = $(LINKER_FLAGS_COMMON) $(LINKER_FLAGS_RELEASE)
!ELSE
COMPILER_FLAGS = $(COMPILER_FLAGS_COMMON) $(COMPILER_FLAGS_DEBUG)
LINKER_FLAGS = $(LINKER_FLAGS_COMMON) $(LINKER_FLAGS_DEBUG)
!ENDIF

!IF $(ARCH) == 86
COMPILER_FLAGS = $(COMPILER_FLAGS) $(COMPILER_FLAGS_86)
LINKER_FLAGS = $(LINKER_FLAGS) $(LINKER_FLAGS_86)
!ELSE
COMPILER_FLAGS = $(COMPILER_FLAGS) $(COMPILER_FLAGS_64)
LINKER_FLAGS = $(LINKER_FLAGS) $(LINKER_FLAGS_64)
!ENDIF

# Real targets.
# Default/build target only compiles and links, but does not run.
_default: build
# Ordering is important so that version is updated first.
build: version.build.txt $(PROJ_BIN)
# Build and run.
run: build
	@ECHO Running $(PROJ_BIN)...
	@CD $(BIN_DIR)$(PROJ_NAME) &&\
		$(BIN_NAME).$(BUILD_NAME).x$(ARCH).exe $(ARGS)
# Clean target removes all intermediates, but remakes the in.txt file.
clean:
	@ECHO Cleaning...
	@IF EXIST $(BIN_DIR) RD /Q /S $(BIN_DIR)
	@IF EXIST $(OBJ_DIR) RD /Q /S $(OBJ_DIR)
# Updates version if any project dependency has changed.
# Mark files as up-to-date after.
# Order dependencies so that output is ordered; not super important.
version.build.txt: $(INCL) $(PROJ_SRC)
	@FOR %%I IN ($?) DO @ECHO Outdated: %%I.
	@version.bat $(uppercase $(REPO_NAME))
	@$(MAKE) /$(MAKEFLAGS) /T version.build.txt

# Build target.
# First compile all outdated files (with /MP), then link everything together.
# Need additional variable to parse dependencies correctly.
$(PROJ_BIN): $(PROJ_SRC)
	@IF NOT EXIST $(PROJ_BIN_DIR) MD $(PROJ_BIN_DIR)
	@IF NOT EXIST $(PROJ_OBJ_DIR) MD $(PROJ_OBJ_DIR)
	@IF NOT EXIST $(PROJ_OBJ_DIR)$(BUILD_NAME) MD $(PROJ_OBJ_DIR)$(BUILD_NAME)
	@IF NOT EXIST $(PROJ_OBJ_DIR)$(BUILD_NAME)\x$(ARCH)\
		MD $(PROJ_OBJ_DIR)$(BUILD_NAME)\x$(ARCH)
	@ECHO Building $(PROJ_BIN)...
	@$(COMPILER) $(COMPILER_FLAGS) $?
	@ECHO Linking $(PROJ_BIN)...
# It's unclear why some of the wildcards need to be escaped, but it works.
	@$(LINKER) $(LINKER_FLAGS) $(patsubsti %.cpp,%.obj,\
		$(patsubsti $(PROJ_SRC_DIR)\%,$(PROJ_OBJ_DIR)$(BUILD_NAME)\x$(ARCH)\\%,$**))

# Mark source files as out of date if any headers have changed.
$(PROJ_SRC): $(INCL)

# ------------------------------------------------------------------------------
!else
else
# ------------------------------------------------------------------------------

# GNU make (gmake) specific commands.
PROJECT ?= experiment
BIN_NAME ?= $(PROJ_NAME)
SRC_NAME ?= $(BIN_NAME).cpp

# For usage of bash commands like `read`.
SHELL := /bin/bash
ROOT_DIR := ../
OBJ_DIR := $(ROOT_DIR)obj/
BIN_DIR := $(ROOT_DIR)bin/
PROJECT_OBJ_DIR := $(OBJ_DIR)$(PROJ_NAME)/
PROJECT_SRC_DIR := $(ROOT_DIR)$(PROJ_NAME)/
PROJ_BIN := $(BIN_DIR)$(PROJ_NAME)/$(BIN_NAME)
PROJ_BIN_DIR := $(BIN_DIR)$(PROJ_NAME)/

COMPILER ?= clang++
COMPILER_FLAGS_INCLUDES := -I $(ROOT_DIR)rain/include -I $(ROOT_DIR)include\
	-I $(ROOT_DIR)\experiment
COMPILER_FLAGS_COMMON := $(COMPILER_FLAGS_INCLUDES) -std=c++20 -pthread\
	-Wall -Wextra -D_CONSOLE -masm=intel
COMPILER_FLAGS_DEBUG := -D _DEBUG -g -O0
COMPILER_FLAGS_RELEASE := -D NDEBUG -Ofast
COMPILER_FLAGS_INSTRUMENT := $(COMPILER_FLAGS_DEBUG) /Fa$(PROJ_OBJ_DIR)
COMPILER_FLAGS_64 := -m64
COMPILER_FLAGS_86 := -m32

LINKER := $(COMPILER)
LINKER_FLAGS_LIBRARIES := -pthread -lresolv
LINKER_FLAGS_COMMON := $(LINKER_FLAGS_LIBRARIES)
LINKER_FLAGS_DEBUG :=
LINKER_FLAGS_RELEASE :=
LINKER_FLAGS_INSTRUMENT := $(LINKER_FLAGS_DEBUG)\
	-fsanitize=address,undefined,leak
LINKER_FLAGS_64 := -m64
LINKER_FLAGS_86 := -m32

# -lstdc++fs is neither available nor required on typical MacOS C++ compiler
# implementations.
ifneq ($(shell uname -s), Darwin)
	LINKER_FLAGS_LIBRARIES := $(LINKER_FLAGS_LIBRARIES) -lstdc++fs
endif

ifeq ($(BUILD), 1)
	COMPILER_FLAGS := $(COMPILER_FLAGS_COMMON) $(COMPILER_FLAGS_RELEASE)
	LINKER_FLAGS := $(LINKER_FLAGS_COMMON) $(LINKER_FLAGS_RELEASE)
else ifeq ($(BUILD), 2)
	COMPILER_FLAGS := $(COMPILER_FLAGS_COMMON) $(COMPILER_FLAGS_INSTRUMENT)
	LINKER_FLAGS := $(LINKER_FLAGS_COMMON) $(LINKER_FLAGS_INSTRUMENT)
else
	COMPILER_FLAGS := $(COMPILER_FLAGS_COMMON) $(COMPILER_FLAGS_DEBUG)
	LINKER_FLAGS := $(LINKER_FLAGS_COMMON) $(LINKER_FLAGS_DEBUG)
endif

ARCH ?= 64
ifeq ($(ARCH), 86)
	COMPILER_FLAGS := $(COMPILER_FLAGS) $(COMPILER_FLAGS_86)
	LINKER_FLAGS := $(LINKER_FLAGS) $(LINKER_FLAGS_86)
else
	COMPILER_FLAGS := $(COMPILER_FLAGS) $(COMPILER_FLAGS_64)
	LINKER_FLAGS := $(LINKER_FLAGS) $(LINKER_FLAGS_64)
endif

# Second expansion for stem matching syntax.
.SECONDEXPANSION:
# Keep intermediate and output files.
# Introduce list of project object intermediates parsed from the source
# directory with SRC_NAME.
PROJECT_OBJS := $(addprefix $(PROJ_OBJ_DIR), $(addsuffix .o,\
	$(basename $(notdir $(wildcard $(PROJ_SRC_DIR)$(SRC_NAME))))))
.SECONDARY: $(PROJ_BIN) $(PROJECT_OBJS)

# Ensure files with the same name as fake targets do not trigger.
.PHONY: __default _default build run clean
_default: build
build: $(PROJ_BIN)
run: build
	@cd $(BIN_DIR)$(PROJ_NAME) && ./$(BIN_NAME) $(ARGS)
# Clean, but recreate in.txt file.
clean:
	@rm -rf $(OBJ_DIR) $(BIN_DIR)

# Link rule.
$(PROJ_BIN): $(BIN_DIR)% : $(PROJECT_OBJS)
	@mkdir -p $(@D)
	@echo $@
	@$(LINKER) $(PROJECT_OBJS) $(LINKER_FLAGS) -o $@

# Compile rule. Stem % matchines source file name.
$(PROJECT_OBJS): $(OBJ_DIR)%.o : $(ROOT_DIR)$$*.cpp
	@mkdir -p $(@D)
	@echo $@
	@$(COMPILER) $(COMPILER_FLAGS) -c $< -o $@

# ------------------------------------------------------------------------------
endif    # gmake: close condition; nmake: not seen
!endif : # gmake: unused target; nmake close conditional
# ------------------------------------------------------------------------------
