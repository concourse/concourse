module Tests exposing (..)

import Array
import ElmTest exposing (..)
import Regex
import String
import StepTreeTests
import PaginationTests
import DurationTests
import JobTests


all : Test
all =
    suite "Concourse"
        [ StepTreeTests.all
        , PaginationTests.all
        , DurationTests.all
        , JobTests.all
        ]
