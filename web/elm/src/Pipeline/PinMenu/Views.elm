module Pipeline.PinMenu.Views exposing
    ( Background(..)
    , Distance(..)
    , Position(..)
    )


type Background
    = Dark
    | Light


type Position
    = TopRight Distance Distance


type Distance
    = Percent Int
    | Px Int
