module Main exposing (main)

import ElmTest exposing (..)
import Tests

main = runSuiteHtml Tests.all
