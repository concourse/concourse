module SearchBar exposing (SearchBar(..), screenSizeChanged)

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


screenSizeChanged :
    { oldSize : ScreenSize
    , newSize : ScreenSize
    }
    -> SearchBar
    -> SearchBar
screenSizeChanged { oldSize, newSize } searchBar =
    case ( searchBar, newSize ) of
        ( Expanded r, _ ) ->
            case ( oldSize, newSize ) of
                ( ScreenSize.Desktop, ScreenSize.Mobile ) ->
                    if String.isEmpty r.query then
                        Collapsed
                    else
                        Expanded r

                _ ->
                    Expanded r

        ( Collapsed, ScreenSize.Desktop ) ->
            Expanded
                { query = ""
                , selectionMade = False
                , showAutocomplete = False
                , selection = 0
                }

        _ ->
            searchBar
