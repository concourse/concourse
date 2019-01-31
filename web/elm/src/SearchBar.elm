module SearchBar exposing (Dropdown(..), SearchBar(..), screenSizeChanged)

import ScreenSize exposing (ScreenSize)


type SearchBar
    = Gone
    | Minified
    | Visible { query : String, dropdown : Dropdown }


type Dropdown
    = Hidden
    | Shown { selectedIdx : Maybe Int }


screenSizeChanged :
    { oldSize : ScreenSize
    , newSize : ScreenSize
    }
    -> SearchBar
    -> SearchBar
screenSizeChanged { oldSize, newSize } searchBar =
    case ( searchBar, newSize ) of
        ( Gone, _ ) ->
            Gone

        ( Visible r, ScreenSize.Mobile ) ->
            case oldSize of
                ScreenSize.Desktop ->
                    if String.isEmpty r.query then
                        Minified

                    else
                        Visible r

                ScreenSize.BigDesktop ->
                    if String.isEmpty r.query then
                        Minified

                    else
                        Visible r

                ScreenSize.Mobile ->
                    Visible r

        ( Visible r, ScreenSize.Desktop ) ->
            Visible r

        ( Visible r, ScreenSize.BigDesktop ) ->
            Visible r

        ( Minified, ScreenSize.Desktop ) ->
            Visible { query = "", dropdown = Hidden }

        ( Minified, ScreenSize.BigDesktop ) ->
            Visible { query = "", dropdown = Hidden }

        ( Minified, ScreenSize.Mobile ) ->
            Minified
