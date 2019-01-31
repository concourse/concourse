module SearchBar exposing (Autocomplete(..), SearchBar(..), screenSizeChanged)

import ScreenSize exposing (ScreenSize)


type SearchBar
    = Collapsed
    | Expanded { query : String, autocomplete : Autocomplete }


type Autocomplete
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
            Expanded { query = "", autocomplete = Hidden }

        ( Collapsed, ScreenSize.BigDesktop ) ->
            Expanded { query = "", autocomplete = Hidden }

        ( Collapsed, ScreenSize.Mobile ) ->
            Collapsed
