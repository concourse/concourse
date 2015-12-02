module Scroll (toBottom, fromBottom, scroll) where

import Task exposing (Task)

import Native.Scroll

toBottom : Task x ()
toBottom =
  Native.Scroll.toBottom ()

fromBottom : Signal Int
fromBottom =
  Native.Scroll.fromBottom

scroll : String -> Float -> Task x ()
scroll =
  Native.Scroll.scroll
