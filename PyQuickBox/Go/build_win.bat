@echo off
setlocal enabledelayedexpansion

set "APP_NAME=PyQuickBox"
set "ICON_ICO=Icon.ico"
set "RC_FILE=pyquickbox.rc"
set "SYSO_FILE=pyquickbox.syso"

echo ----------------------------------------------------
echo  PyQuickBox Native Builder (MinGW/windres)
echo ----------------------------------------------------

:: Cleanup
if exist %SYSO_FILE% del %SYSO_FILE%
if exist %APP_NAME%.exe del %APP_NAME%.exe
if exist %RC_FILE% del %RC_FILE%

:: 1. Check Build Environment
echo [CHECK] Checking Go...
where go >nul 2>nul
if %errorlevel% neq 0 goto :NoGo

echo [CHECK] Checking GCC...
where gcc >nul 2>nul
if %errorlevel% neq 0 goto :NoGCC

echo [CHECK] Checking windres (Resource Compiler)...
where windres >nul 2>nul
if %errorlevel% neq 0 (
    echo [WARNING] windres not found. Falling back to rsrc...
    goto :UseRsrc
)

:UseWindres
:: Check if Icon exists
if not exist %ICON_ICO% goto :NoIcon

echo.
echo [STEP 1/3] Creating Resource Script...
:: Create .rc file
echo 100 ICON "%ICON_ICO%" > %RC_FILE%

echo [STEP 2/3] Compiling Resource with windres...
:: Compile .rc to .syso (COFF format for Go linker)
windres -i %RC_FILE% -O coff -o %SYSO_FILE%
if %errorlevel% neq 0 (
    echo [ERROR] windres failed to compile the icon.
    echo Please ensure Icon.ico is a valid Windows Icon file.
    echo Recommended size: 256x256 pixels.
    pause
    goto :BuildNoIcon
)
echo [INFO] Resource compiled successfully (%SYSO_FILE%).
goto :Build

:UseRsrc
if not exist %ICON_ICO% goto :NoIcon
echo.
echo [STEP 1/3] Using rsrc (Fallback)...
where rsrc >nul 2>nul
if %errorlevel% neq 0 go install github.com/akavel/rsrc@latest
rsrc -ico %ICON_ICO% -o %SYSO_FILE%
if %errorlevel% neq 0 goto :BuildNoIcon
goto :Build

:Build
echo.
echo [STEP 3/3] Building Application...
go mod tidy
:: -H=windowsgui hides console, -s -w strips symbols
go build -ldflags="-H=windowsgui -s -w" -v -o %APP_NAME%.exe .

if %errorlevel% equ 0 goto :Success
goto :Fail

:BuildNoIcon
echo.
echo [WARNING] Building WITHOUT icon due to previous errors...
go mod tidy
go build -ldflags="-H=windowsgui -s -w" -v -o %APP_NAME%.exe .
if %errorlevel% equ 0 goto :Success
goto :Fail

:NoIcon
echo.
echo [WARNING] %ICON_ICO% not found per user request.
echo Building without icon...
goto :BuildNoIcon

:Success
echo.
echo ----------------------------------------------------
echo [SUCCESS] Build Complete: %APP_NAME%.exe
echo ----------------------------------------------------
if exist %SYSO_FILE% (
    echo [INFO] Icon embedded via %SYSO_FILE%
    echo Note: Windows Explorer caches icons. 
    echo Move the .exe to a new folder to see the icon change immediately.
)
:: Cleanup
if exist %RC_FILE% del %RC_FILE%
if exist %SYSO_FILE% del %SYSO_FILE%
pause
exit /b 0

:NoGo
echo [ERROR] Go not installed.
pause
exit /b 1

:NoGCC
echo [ERROR] GCC not installed.
pause
exit /b 1

:Fail
echo [FAILURE] Build Failed.
pause
exit /b 1
