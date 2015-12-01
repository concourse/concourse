module Tests where

import Array
import ElmTest exposing (..)
import Regex
import String

import StepTreeTests
import PaginationTests
import DurationTests

all : Test
all =
  suite "Concourse"
    [ StepTreeTests.all
    , PaginationTests.all
    , DurationTests.all
    ]
