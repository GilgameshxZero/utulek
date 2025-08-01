@REM Proxy file that calls nmake with INCL set to all header directories.
@ECHO OFF
SETLOCAL ENABLEDELAYEDEXPANSION

@REM Careful to not exceed the 8192 string limit.
SET "INCL=..\rain\include\* ..\include\*"
FOR /F "delims=" %%I IN ('DIR /B /S /AD ..\rain\include ..\include') DO (
	SET "INCL=%%I\* !INCL!"
)

@REM Suppresses CMD terminate prompt.
nmake /C %* || CALL;
ENDLOCAL
