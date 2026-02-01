module Views.SearchBar exposing
    ( Dropdown(..)
    , HandleDeliveryConfig
    , Suggestion
    , UpdateConfig
    , ViewConfig
    , handleDelivery
    , searchInputId
    , update
    , view
    )

import Dashboard.Styles as Styles
import EffectTransformer exposing (ET)
import Html exposing (Html)
import Html.Attributes exposing (attribute, id, placeholder, style, value)
import Html.Events exposing (onBlur, onClick, onFocus, onInput, onMouseDown)
import Keyboard
import List.Extra
import Message.Effects exposing (Effect(..))
import Message.Message exposing (DomID(..), Message(..))
import Message.Subscription exposing (Delivery(..))
import ScreenSize exposing (ScreenSize(..))


searchInputId : String
searchInputId =
    "search-input-field"


type Dropdown
    = Hidden
    | Shown (Maybe Int)


type alias UpdateConfig =
    { screenSize : ScreenSize
    , highDensity : Bool
    , clearEffects : List Effect
    , changeEffects : String -> List Effect
    }


update : UpdateConfig -> Message -> ET { a | query : String, dropdown : Dropdown }
update config msg ( model, effects ) =
    case msg of
        Click ShowSearchButton ->
            showSearchInput config ( model, effects )

        Click ClearSearchButton ->
            ( { model | query = "" }
            , effects
                ++ (Focus searchInputId :: config.clearEffects)
            )

        FilterMsg query ->
            ( { model | query = query }
            , effects
                ++ (Focus searchInputId :: config.changeEffects query)
            )

        FocusMsg ->
            ( { model | dropdown = Shown Nothing }, effects )

        BlurMsg ->
            ( { model | dropdown = Hidden }, effects )

        _ ->
            ( model, effects )


showSearchInput : UpdateConfig -> ET { a | query : String, dropdown : Dropdown }
showSearchInput config ( model, effects ) =
    if config.highDensity then
        ( model, effects )

    else
        let
            isDropDownHidden =
                model.dropdown == Hidden

            isMobile =
                config.screenSize == Mobile
        in
        if isDropDownHidden && isMobile && model.query == "" then
            ( { model | dropdown = Shown Nothing }
            , effects ++ [ Focus searchInputId ]
            )

        else
            ( model, effects )


type alias Suggestion =
    { prev : String
    , cur : String
    }


type alias HandleDeliveryConfig model =
    { suggestions : model -> List Suggestion
    , selectEffects : String -> List Effect
    }


handleDelivery : HandleDeliveryConfig { a | query : String, dropdown : Dropdown } -> Delivery -> ET { a | query : String, dropdown : Dropdown }
handleDelivery config delivery ( model, effects ) =
    case delivery of
        WindowResized width _ ->
            ( screenResize width model, effects )

        KeyDown keyEvent ->
            let
                options =
                    config.suggestions model
            in
            case keyEvent.code of
                Keyboard.ArrowUp ->
                    ( { model
                        | dropdown =
                            arrowUp options model.dropdown
                      }
                    , effects
                    )

                Keyboard.ArrowDown ->
                    ( { model
                        | dropdown =
                            arrowDown options model.dropdown
                      }
                    , effects
                    )

                Keyboard.Enter ->
                    case model.dropdown of
                        Shown (Just idx) ->
                            let
                                selectedItem =
                                    options
                                        |> List.Extra.getAt idx
                                        |> Maybe.map (\{ prev, cur } -> prev ++ cur)
                                        |> Maybe.withDefault model.query
                            in
                            ( { model
                                | dropdown = Shown Nothing
                                , query = selectedItem
                              }
                            , config.selectEffects selectedItem
                            )

                        Shown Nothing ->
                            ( model, effects )

                        Hidden ->
                            ( model, effects )

                Keyboard.Escape ->
                    ( model, effects ++ [ Blur searchInputId ] )

                Keyboard.Slash ->
                    ( model
                    , if keyEvent.shiftKey then
                        effects

                      else
                        effects ++ [ Focus searchInputId ]
                    )

                _ ->
                    ( model, effects )

        _ ->
            ( model, effects )


screenResize : Float -> { a | dropdown : Dropdown } -> { a | dropdown : Dropdown }
screenResize width model =
    let
        newSize =
            ScreenSize.fromWindowSize width
    in
    case newSize of
        Desktop ->
            { model | dropdown = Hidden }

        BigDesktop ->
            { model | dropdown = Hidden }

        Mobile ->
            model


ignoreEmpty : (List a -> b -> b) -> List a -> b -> b
ignoreEmpty fn l =
    if List.isEmpty l then
        identity

    else
        fn l


arrowUp : List a -> Dropdown -> Dropdown
arrowUp =
    ignoreEmpty
        (\options dropdown ->
            case dropdown of
                Shown Nothing ->
                    let
                        lastItem =
                            List.length options - 1
                    in
                    Shown (Just lastItem)

                Shown (Just idx) ->
                    let
                        newSelection =
                            modBy (List.length options) (idx - 1)
                    in
                    Shown (Just newSelection)

                Hidden ->
                    Hidden
        )


arrowDown : List a -> Dropdown -> Dropdown
arrowDown =
    ignoreEmpty
        (\options dropdown ->
            case dropdown of
                Shown Nothing ->
                    Shown (Just 0)

                Shown (Just idx) ->
                    let
                        newSelection =
                            modBy (List.length options) (idx + 1)
                    in
                    Shown (Just newSelection)

                Hidden ->
                    Hidden
        )


type alias ViewConfig model =
    { screenSize : ScreenSize
    , highDensity : Bool
    , placeholder : String
    , disabled : Bool
    , dropdownAbsolute : Bool
    , suggestions : model -> List Suggestion
    }


view : ViewConfig { a | query : String, dropdown : Dropdown } -> { a | query : String, dropdown : Dropdown } -> Html Message
view config ({ query, dropdown } as model) =
    let
        isDropDownHidden =
            dropdown == Hidden

        isMobile =
            config.screenSize == Mobile

        clearSearchButton =
            if String.length query > 0 then
                [ Html.div
                    ([ id "search-clear"
                     , onClick <| Click ClearSearchButton
                     ]
                        ++ Styles.searchClearButton
                    )
                    []
                ]

            else
                []
    in
    if config.disabled then
        Html.text ""

    else if isDropDownHidden && isMobile && query == "" then
        Html.div
            (Styles.showSearchContainer
                { screenSize = config.screenSize
                , highDensity = config.highDensity
                }
            )
            [ Html.div
                ([ id "show-search-button"
                 , onClick <| Click ShowSearchButton
                 ]
                    ++ Styles.searchButton
                )
                []
            ]

    else
        Html.div
            (id "search-container" :: Styles.searchContainer config.screenSize)
            (Html.input
                ([ id searchInputId
                 , placeholder config.placeholder
                 , attribute "autocomplete" "off"
                 , value query
                 , onFocus FocusMsg
                 , onBlur BlurMsg
                 , onInput FilterMsg
                 ]
                    ++ Styles.searchInput
                        config.screenSize
                        (String.length query > 0)
                )
                []
                :: clearSearchButton
                ++ viewDropdownItems config model
            )


viewDropdownItems : ViewConfig { a | query : String, dropdown : Dropdown } -> { a | query : String, dropdown : Dropdown } -> List (Html Message)
viewDropdownItems config model =
    case model.dropdown of
        Hidden ->
            []

        Shown selectedIdx ->
            let
                dropdownItem : Int -> Suggestion -> Html Message
                dropdownItem idx { prev, cur } =
                    Html.li
                        (onMouseDown (FilterMsg <| prev ++ cur)
                            :: Styles.dropdownItem
                                (Just idx == selectedIdx)
                                (String.length model.query > 0)
                        )
                        [ Html.text cur ]
            in
            [ Html.ul
                ((id "search-dropdown"
                    :: (if config.dropdownAbsolute then
                            [ style "position" "absolute"
                            , style "z-index" "1000"
                            ]

                        else
                            []
                       )
                 )
                    ++ Styles.dropdownContainer config.screenSize
                )
                (List.indexedMap dropdownItem (config.suggestions model))
            ]
