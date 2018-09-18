module Favicon
    exposing
        ( set
        )

import Task exposing (Task)
import Native.Favicon


set : String -> Task x ()
set =
    Native.Favicon.set
