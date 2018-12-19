module SearchBar exposing (SearchBar(..), screenSizeChanged)

import ScreenSize exposing (ScreenSize)


type SearchBar
    = Collapsed
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
        ( Expanded r, ScreenSize.Mobile ) ->
            case oldSize of
                ScreenSize.Desktop ->
                    if String.isEmpty r.query then
                        Collapsed
                    else
                        Expanded r

                ScreenSize.BigDesktop ->
                    if String.isEmpty r.query then
                        Collapsed
                    else
                        Expanded r

                ScreenSize.Mobile ->
                    Expanded r

        ( Expanded r, ScreenSize.Desktop ) ->
            Expanded r

        ( Expanded r, ScreenSize.BigDesktop ) ->
            Expanded r

        ( Collapsed, ScreenSize.Desktop ) ->
            Expanded
                { query = ""
                , selectionMade = False
                , showAutocomplete = False
                , selection = 0
                }

        ( Collapsed, ScreenSize.BigDesktop ) ->
            Expanded
                { query = ""
                , selectionMade = False
                , showAutocomplete = False
                , selection = 0
                }

        ( Collapsed, ScreenSize.Mobile ) ->
            Collapsed
