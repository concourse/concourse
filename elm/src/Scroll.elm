module Scroll (toBottom, fromBottom, scroll, scrollIntoView) where

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

scrollIntoView : String -> Task x ()
scrollIntoView =
  Native.Scroll.scrollIntoView
