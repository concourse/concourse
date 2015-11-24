module Scroll (toBottom, fromBottom) where

import Task exposing (Task)

import Native.Scroll

toBottom : Task x ()
toBottom =
  Native.Scroll.toBottom ()

fromBottom : Signal Int
fromBottom =
  Native.Scroll.fromBottom
