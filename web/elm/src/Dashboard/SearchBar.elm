module Dashboard.SearchBar exposing
    ( handleDelivery
    , searchInputId
    , update
    , view
    )

import Application.Models exposing (Session)
import Concourse.PipelineStatus
    exposing
        ( PipelineStatus(..)
        , StatusDetails(..)
        )
import Dashboard.Filter as Filter
import Dashboard.Models exposing (Dropdown(..), Model)
import Dashboard.Styles as Styles
import Dict
import EffectTransformer exposing (ET)
import Html exposing (Html)
import Html.Attributes exposing (attribute, id, placeholder, value)
import Html.Events exposing (onBlur, onClick, onFocus, onInput, onMouseDown)
import Keyboard
import List.Extra
import Message.Callback exposing (Callback(..))
import Message.Effects exposing (Effect(..))
import Message.Message exposing (DomID(..), Message(..))
import Message.Subscription exposing (Delivery(..))
import Routes
import ScreenSize exposing (ScreenSize)


searchInputId : String
searchInputId =
    "search-input-field"


update : Session -> Message -> ET Model
update session msg ( model, effects ) =
    case msg of
        Click ShowSearchButton ->
            showSearchInput session ( model, effects )

        Click ClearSearchButton ->
            ( { model | query = "" }
            , effects
                ++ [ Focus searchInputId
                   , ModifyUrl <|
                        Routes.toString <|
                            Routes.Dashboard
                                { searchType = Routes.Normal ""
                                , dashboardView = model.dashboardView
                                }
                   ]
            )

        FilterMsg query ->
            ( { model | query = query }
            , effects
                ++ [ Focus searchInputId
                   , ModifyUrl <|
                        Routes.toString <|
                            Routes.Dashboard
                                { searchType = Routes.Normal query
                                , dashboardView = model.dashboardView
                                }
                   ]
            )

        FocusMsg ->
            ( { model | dropdown = Shown Nothing }, effects )

        BlurMsg ->
            ( { model | dropdown = Hidden }, effects )

        _ ->
            ( model, effects )


showSearchInput : { a | screenSize : ScreenSize } -> ET Model
showSearchInput session ( model, effects ) =
    if model.highDensity then
        ( model, effects )

    else
        let
            isDropDownHidden =
                model.dropdown == Hidden

            isMobile =
                session.screenSize == ScreenSize.Mobile
        in
        if isDropDownHidden && isMobile && model.query == "" then
            ( { model | dropdown = Shown Nothing }
            , effects ++ [ Focus searchInputId ]
            )

        else
            ( model, effects )


screenResize : Float -> Model -> Model
screenResize width model =
    let
        newSize =
            ScreenSize.fromWindowSize width
    in
    case newSize of
        ScreenSize.Desktop ->
            { model | dropdown = Hidden }

        ScreenSize.BigDesktop ->
            { model | dropdown = Hidden }

        ScreenSize.Mobile ->
            model


handleDelivery : Session -> Delivery -> ET Model
handleDelivery session delivery ( model, effects ) =
    case delivery of
        WindowResized width _ ->
            ( screenResize width model, effects )

        KeyDown keyEvent ->
            let
                options =
                    Filter.suggestions (Filter.filterTeams session model) model.query
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
                            , [ ModifyUrl <|
                                    Routes.toString <|
                                        Routes.Dashboard
                                            { searchType = Routes.Normal selectedItem
                                            , dashboardView = model.dashboardView
                                            }
                              ]
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

                -- any other keycode
                _ ->
                    ( model, effects )

        _ ->
            ( model, effects )


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


view : Session -> Model -> Html Message
view session ({ query, dropdown, pipelines } as params) =
    let
        isDropDownHidden =
            dropdown == Hidden

        isMobile =
            session.screenSize == ScreenSize.Mobile

        noPipelines =
            pipelines
                |> Maybe.withDefault Dict.empty
                |> Dict.values
                |> List.all List.isEmpty

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
    if noPipelines then
        Html.text ""

    else if isDropDownHidden && isMobile && query == "" then
        Html.div
            (Styles.showSearchContainer
                { screenSize = session.screenSize
                , highDensity = params.highDensity
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
            (id "search-container" :: Styles.searchContainer session.screenSize)
            (Html.input
                ([ id searchInputId
                 , placeholder "filter pipelines by name, status, or team"
                 , attribute "autocomplete" "off"
                 , value query
                 , onFocus FocusMsg
                 , onBlur BlurMsg
                 , onInput FilterMsg
                 ]
                    ++ Styles.searchInput
                        session.screenSize
                        (String.length query > 0)
                )
                []
                :: clearSearchButton
                ++ viewDropdownItems session params
            )


viewDropdownItems : Session -> Model -> List (Html Message)
viewDropdownItems session model =
    case model.dropdown of
        Hidden ->
            []

        Shown selectedIdx ->
            let
                dropdownItem : Int -> Filter.Suggestion -> Html Message
                dropdownItem idx { prev, cur } =
                    Html.li
                        (onMouseDown (FilterMsg <| prev ++ cur)
                            :: Styles.dropdownItem
                                (Just idx == selectedIdx)
                                (String.length model.query > 0)
                        )
                        [ Html.text cur ]

                filteredTeams =
                    Filter.filterTeams session model
            in
            [ Html.ul
                (id "search-dropdown" :: Styles.dropdownContainer session.screenSize)
                (List.indexedMap dropdownItem (Filter.suggestions filteredTeams model.query))
            ]
