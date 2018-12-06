module SearchBar exposing (SearchBar(..))

import ScreenSize exposing (ScreenSize)


type SearchBar
    = Invisible
    | Collapsed
    | Expanded
        { query : String
        , showAutocomplete : Bool
        , selectionMade : Bool
        , selection : Int
        }
