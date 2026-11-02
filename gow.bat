@echo off
rem go.jar wrapper: the one command a Go mod project needs. Requires Java 25.
cd /d "%~dp0"
if defined JAVA_HOME (set "JAVA=%JAVA_HOME%\bin\java") else (set "JAVA=java")
"%JAVA%" -jar go.jar %*
