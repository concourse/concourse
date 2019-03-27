module ScreenSize exposing (ScreenSize(..), fromWindowSize)


type ScreenSize
    = Mobile
    | Desktop
    | BigDesktop


fromWindowSize : Float -> Float -> ScreenSize
fromWindowSize width height =
    if width < 812 then
        Mobile

    else if width < 1230 then
        Desktop

    else
        BigDesktop
