module Favicon exposing (set)

import Native.Favicon
import Task exposing (Task)


set : String -> Task x ()
set =
    Native.Favicon.set
