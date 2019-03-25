module Dashboard.SearchBar exposing (handleDelivery, searchInputId, update, view)

import Array
import Dashboard.Group.Models exposing (Group)
import Dashboard.Models exposing (Dropdown(..), Model)
import Dashboard.Styles as Styles
import EffectTransformer exposing (ET)
import Html exposing (Html)
import Html.Attributes exposing (attribute, id, placeholder, style, value)
import Html.Events exposing (onBlur, onClick, onFocus, onInput, onMouseDown)
import Keycodes
import Message.Callback exposing (Callback(..))
import Message.Effects exposing (Effect(..))
import Message.Message exposing (Message(..))
import Message.Subscription exposing (Delivery(..))
import Routes
import ScreenSize exposing (ScreenSize)
import Window


searchInputId : String
searchInputId =
    "search-input-field"


update : Message -> ET Model
update msg ( model, effects ) =
    case msg of
        ShowSearchInput ->
            showSearchInput ( model, effects )

        FilterMsg query ->
            ( { model | query = query }
            , effects
                ++ [ Focus searchInputId
                   , ModifyUrl <|
                        Routes.toString <|
                            Routes.Dashboard (Routes.Normal (Just query))
                   ]
            )

        FocusMsg ->
            let
                newModel =
                    { model | dropdown = Shown { selectedIdx = Nothing } }
            in
            ( newModel, effects )

        BlurMsg ->
            let
                newModel =
                    { model | dropdown = Hidden }
            in
            ( newModel, effects )

        _ ->
            ( model, effects )


showSearchInput : ET Model
showSearchInput ( model, effects ) =
    if model.highDensity then
        ( model, effects )

    else
        let
            isDropDownHidden =
                model.dropdown == Hidden

            isMobile =
                model.screenSize == ScreenSize.Mobile

            newModel =
                { model | dropdown = Shown { selectedIdx = Nothing } }
        in
        if isDropDownHidden && isMobile && model.query == "" then
            ( newModel, effects ++ [ Focus searchInputId ] )

        else
            ( model, effects )


screenResize : Window.Size -> Model -> Model
screenResize size model =
    let
        newSize =
            ScreenSize.fromWindowSize size

        newModel =
            { model | screenSize = newSize }
    in
    case newSize of
        ScreenSize.Desktop ->
            { newModel | dropdown = Hidden }

        ScreenSize.BigDesktop ->
            { newModel | dropdown = Hidden }

        ScreenSize.Mobile ->
            newModel


handleCallback : Callback -> ET Model
handleCallback callback ( model, effects ) =
    case callback of
        ScreenResized size ->
            ( screenResize size model, effects )

        _ ->
            ( model, effects )


handleDelivery : Delivery -> ET Model
handleDelivery delivery ( model, effects ) =
    case delivery of
        WindowResized size ->
            ( screenResize size model, effects )

        KeyUp keyCode ->
            if keyCode == Keycodes.shift then
                ( { model | shiftDown = False }, effects )

            else
                ( model, effects )

        KeyDown keyCode ->
            if keyCode == Keycodes.shift then
                ( { model | shiftDown = True }, effects )

            else
                let
                    options =
                        dropdownOptions model
                in
                case keyCode of
                    -- up arrow
                    38 ->
                        ( { model
                            | dropdown =
                                arrowUp options model.dropdown
                          }
                        , effects
                        )

                    -- down arrow
                    40 ->
                        ( { model
                            | dropdown =
                                arrowDown options model.dropdown
                          }
                        , effects
                        )

                    -- enter key
                    13 ->
                        case model.dropdown of
                            Shown { selectedIdx } ->
                                case selectedIdx of
                                    Nothing ->
                                        ( model, effects )

                                    Just selectedIdx ->
                                        let
                                            options =
                                                Array.fromList <|
                                                    dropdownOptions model

                                            selectedItem =
                                                Array.get selectedIdx options
                                                    |> Maybe.withDefault
                                                        model.query
                                        in
                                        ( { model
                                            | dropdown = Shown { selectedIdx = Nothing }
                                            , query = selectedItem
                                          }
                                        , [ ModifyUrl <|
                                                Routes.toString <|
                                                    Routes.Dashboard (Routes.Normal (Just selectedItem))
                                          ]
                                        )

                            _ ->
                                ( model, effects )

                    -- escape key
                    27 ->
                        ( model, effects ++ [ Blur searchInputId ] )

                    -- '/'
                    191 ->
                        ( model
                        , if model.shiftDown then
                            effects

                          else
                            effects ++ [ Focus searchInputId ]
                        )

                    -- any other keycode
                    _ ->
                        ( model, effects )

        _ ->
            ( model, effects )


arrowUp : List a -> Dropdown -> Dropdown
arrowUp options dropdown =
    case dropdown of
        Shown { selectedIdx } ->
            case selectedIdx of
                Nothing ->
                    let
                        lastItem =
                            List.length options - 1
                    in
                    Shown { selectedIdx = Just lastItem }

                Just selectedIdx ->
                    let
                        newSelection =
                            (selectedIdx - 1) % List.length options
                    in
                    Shown { selectedIdx = Just newSelection }

        Hidden ->
            Hidden


arrowDown : List a -> Dropdown -> Dropdown
arrowDown options dropdown =
    case dropdown of
        Shown { selectedIdx } ->
            case selectedIdx of
                Nothing ->
                    Shown { selectedIdx = Just 0 }

                Just selectedIdx ->
                    let
                        newSelection =
                            (selectedIdx + 1) % List.length options
                    in
                    Shown { selectedIdx = Just newSelection }

        Hidden ->
            Hidden


view :
    { a
        | screenSize : ScreenSize
        , query : String
        , dropdown : Dropdown
        , groups : List Group
        , highDensity : Bool
    }
    -> Html Message
view ({ screenSize, query, dropdown, groups } as params) =
    let
        isDropDownHidden =
            dropdown == Hidden

        isMobile =
            screenSize == ScreenSize.Mobile

        noPipelines =
            groups
                |> List.concatMap .pipelines
                |> List.isEmpty
    in
    if noPipelines then
        Html.text ""

    else if isDropDownHidden && isMobile && query == "" then
        Html.div
            [ style <|
                Styles.showSearchContainer params
            ]
            [ Html.a
                [ id "show-search-button"
                , onClick ShowSearchInput
                , style Styles.searchButton
                ]
                []
            ]

    else
        Html.div
            [ id "search-container"
            , style (Styles.searchContainer screenSize)
            ]
            ([ Html.input
                [ id searchInputId
                , style (Styles.searchInput screenSize)
                , placeholder "search"
                , attribute "autocomplete" "off"
                , value query
                , onFocus FocusMsg
                , onBlur BlurMsg
                , onInput FilterMsg
                ]
                []
             , Html.div
                [ id "search-clear"
                , onClick (FilterMsg "")
                , style (Styles.searchClearButton (String.length query > 0))
                ]
                []
             ]
                ++ viewDropdownItems params
            )


viewDropdownItems :
    { a
        | query : String
        , dropdown : Dropdown
        , groups : List Group
        , screenSize : ScreenSize
    }
    -> List (Html Message)
viewDropdownItems ({ dropdown, screenSize } as model) =
    case dropdown of
        Hidden ->
            []

        Shown { selectedIdx } ->
            let
                dropdownItem : Int -> String -> Html Message
                dropdownItem idx text =
                    Html.li
                        [ onMouseDown (FilterMsg text)
                        , style (Styles.dropdownItem (Just idx == selectedIdx))
                        ]
                        [ Html.text text ]
            in
            [ Html.ul
                [ id "search-dropdown"
                , style (Styles.dropdownContainer screenSize)
                ]
                (List.indexedMap dropdownItem (dropdownOptions model))
            ]


dropdownOptions : { a | query : String, groups : List Group } -> List String
dropdownOptions { query, groups } =
    case String.trim query of
        "" ->
            [ "status: ", "team: " ]

        "status:" ->
            [ "status: paused"
            , "status: pending"
            , "status: failed"
            , "status: errored"
            , "status: aborted"
            , "status: running"
            , "status: succeeded"
            ]

        "team:" ->
            groups
                |> List.take 10
                |> List.map (\group -> "team: " ++ group.teamName)

        _ ->
            []
