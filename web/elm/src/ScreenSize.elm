module ScreenSize exposing (ScreenSize(..), fromWindowSize)

import Window


type ScreenSize
    = Mobile
    | Desktop
    | BigDesktop


fromWindowSize : Window.Size -> ScreenSize
fromWindowSize size =
    if size.width < 812 then
        Mobile
    else if size.width < 1230 then
        Desktop
    else
        BigDesktop
