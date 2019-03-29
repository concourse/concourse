module Dashboard.SearchBar exposing (handleDelivery, searchInputId, update, view)

import Array
import Dashboard.Group.Models exposing (Group)
import Dashboard.Models exposing (Dropdown(..), Model)
import Dashboard.Styles as Styles
import EffectTransformer exposing (ET)
import Html exposing (Html)
import Html.Attributes exposing (attribute, id, placeholder, value)
import Html.Events exposing (onBlur, onClick, onFocus, onInput, onMouseDown)
import Keyboard
import Message.Callback exposing (Callback(..))
import Message.Effects exposing (Effect(..))
import Message.Message exposing (Message(..))
import Message.Subscription exposing (Delivery(..))
import Routes
import ScreenSize exposing (ScreenSize)


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


screenResize : Float -> Float -> Model -> Model
screenResize width height model =
    let
        newSize =
            ScreenSize.fromWindowSize width height

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
        ScreenResized viewport ->
            ( screenResize
                viewport.viewport.width
                viewport.viewport.height
                model
            , effects
            )

        _ ->
            ( model, effects )


handleDelivery : Delivery -> ET Model
handleDelivery delivery ( model, effects ) =
    case delivery of
        WindowResized width height ->
            ( screenResize width height model, effects )

        KeyDown keyEvent ->
            let
                options =
                    dropdownOptions model
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
                        Shown { selectedIdx } ->
                            case selectedIdx of
                                Nothing ->
                                    ( model, effects )

                                Just idx ->
                                    let
                                        selectedItem =
                                            options
                                                |> Array.fromList
                                                |> Array.get idx
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

                Keyboard.Escape ->
                    ( model, effects ++ [ Blur searchInputId ] )

                Keyboard.Slash ->
                    ( model
                    , if keyEvent.shiftKey then
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

                Just idx ->
                    let
                        newSelection =
                            modBy (List.length options) (idx - 1)
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

                Just idx ->
                    let
                        newSelection =
                            modBy (List.length options) (idx + 1)
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
            (Styles.showSearchContainer params)
            [ Html.a
                ([ id "show-search-button"
                 , onClick ShowSearchInput
                 ]
                    ++ Styles.searchButton
                )
                []
            ]

    else
        Html.div
            ([ id "search-container" ]
                ++ Styles.searchContainer screenSize
            )
            ([ Html.input
                ([ id searchInputId
                 , placeholder "search"
                 , attribute "autocomplete" "off"
                 , value query
                 , onFocus FocusMsg
                 , onBlur BlurMsg
                 , onInput FilterMsg
                 ]
                    ++ Styles.searchInput screenSize
                )
                []
             , Html.div
                ([ id "search-clear"
                 , onClick (FilterMsg "")
                 ]
                    ++ Styles.searchClearButton (String.length query > 0)
                )
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
                        ([ onMouseDown (FilterMsg text) ]
                            ++ Styles.dropdownItem (Just idx == selectedIdx)
                        )
                        [ Html.text text ]
            in
            [ Html.ul
                ([ id "search-dropdown" ]
                    ++ Styles.dropdownContainer screenSize
                )
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
