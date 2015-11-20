module Tests where

import Array
import ElmTest exposing (..)
import Regex
import String

import StepTreeTests

all : Test
all =
  suite "Concourse"
    [ StepTreeTests.all
    ]
