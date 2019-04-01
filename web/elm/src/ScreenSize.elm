module ScreenSize exposing (ScreenSize(..), fromWindowSize)


type ScreenSize
    = Mobile
    | Desktop
    | BigDesktop


fromWindowSize : Float -> ScreenSize
fromWindowSize width =
    if width < 812 then
        Mobile

    else if width < 1230 then
        Desktop

    else
        BigDesktop
