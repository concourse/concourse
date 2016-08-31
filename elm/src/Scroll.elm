module Scroll exposing
  ( toBottom
  , fromBottom
  , scroll
  , scrollIntoView
  )

import Task exposing (Task)

import Native.Scroll

toBottom : String -> Task x ()
toBottom =
  Native.Scroll.toBottom

scroll : String -> Float -> Task x ()
scroll =
  Native.Scroll.scrollElement

scrollIntoView : String -> Task x ()
scrollIntoView =
  Native.Scroll.scrollIntoView

fromBottom : String -> Task x Int
fromBottom =
  Native.Scroll.fromBottom
