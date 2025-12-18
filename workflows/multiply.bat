@echo off
echo Starting batch script...
if "%INPUT_VALUE%"=="" (
    set INPUT_VALUE=%1
)
echo Input: %INPUT_VALUE%
set /a result=%INPUT_VALUE% * 3
echo Result: %result%
echo OUTPUT_RESULT:%result%
echo Batch script completed.