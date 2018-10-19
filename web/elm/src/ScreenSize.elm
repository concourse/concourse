module ScreenSize exposing (ScreenSize(..), getScreenSize)

import Window


type ScreenSize
    = Mobile
    | Desktop


getScreenSize : Window.Size -> ScreenSize
getScreenSize size =
    if size.width < 812 then
        Mobile
    else
        Desktop
