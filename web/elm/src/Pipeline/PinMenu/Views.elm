module Pipeline.PinMenu.Views exposing
    ( Background(..)
    , Brightness(..)
    , Distance(..)
    , Position(..)
    )


type Background
    = Dark
    | Light


type Brightness
    = Bright
    | Dim


type Position
    = TopRight Distance Distance


type Distance
    = Percent Int
    | Px Int
