module Redirect exposing
  ( to
  )

import Task exposing (Task)

import Native.Redirect


to : String -> Task x ()
to =
  Native.Redirect.to
