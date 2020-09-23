module Dashboard.SearchBar exposing
    ( handleDelivery
    , searchInputId
    , update
    , view
    )

import Application.Models exposing (Session)
import Array
import Concourse
import Concourse.PipelineStatus
    exposing
        ( PipelineStatus(..)
        , StatusDetails(..)
        )
import Dashboard.Group.Models exposing (Pipeline)
import Dashboard.Models exposing (Dropdown(..), Model)
import Dashboard.Styles as Styles
import Dict exposing (Dict)
import EffectTransformer exposing (ET)
import FetchResult exposing (FetchResult)
import Html exposing (Html)
import Html.Attributes exposing (attribute, id, placeholder, value)
import Html.Events exposing (onBlur, onClick, onFocus, onInput, onMouseDown)
import Keyboard
import Message.Callback exposing (Callback(..))
import Message.Effects exposing (Effect(..))
import Message.Message exposing (DomID(..), Message(..))
import Message.Subscription exposing (Delivery(..))
import Routes
import ScreenSize exposing (ScreenSize)
import Set


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
                                { searchType = Routes.Normal "" model.instanceGroup
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
                                { searchType = Routes.Normal query model.instanceGroup
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


handleDelivery : Delivery -> ET Model
handleDelivery delivery ( model, effects ) =
    case delivery of
        WindowResized width _ ->
            ( screenResize width model, effects )

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
                        Shown (Just idx) ->
                            let
                                selectedItem =
                                    options
                                        |> Array.fromList
                                        |> Array.get idx
                                        |> Maybe.withDefault
                                            model.query
                            in
                            ( { model
                                | dropdown = Shown Nothing
                                , query = selectedItem
                              }
                            , [ ModifyUrl <|
                                    Routes.toString <|
                                        Routes.Dashboard
                                            { searchType = Routes.Normal selectedItem model.instanceGroup
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


arrowUp : List a -> Dropdown -> Dropdown
arrowUp options dropdown =
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


arrowDown : List a -> Dropdown -> Dropdown
arrowDown options dropdown =
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


view :
    { a | screenSize : ScreenSize }
    ->
        { b
            | query : String
            , dropdown : Dropdown
            , teams : FetchResult (List Concourse.Team)
            , highDensity : Bool
            , pipelines : Maybe (Dict String (List Pipeline))
        }
    -> Html Message
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
            ([ Html.input
                ([ id searchInputId
                 , placeholder "search"
                 , attribute "autocomplete" "off"
                 , value query
                 , onFocus FocusMsg
                 , onBlur BlurMsg
                 , onInput FilterMsg
                 ]
                    ++ Styles.searchInput session.screenSize
                )
                []
             , Html.div
                ([ id "search-clear"
                 , onClick <| Click ClearSearchButton
                 ]
                    ++ Styles.searchClearButton (String.length query > 0)
                )
                []
             ]
                ++ viewDropdownItems session params
            )


viewDropdownItems :
    { a
        | screenSize : ScreenSize
    }
    ->
        { b
            | query : String
            , dropdown : Dropdown
            , teams : FetchResult (List Concourse.Team)
            , pipelines : Maybe (Dict String (List Pipeline))
        }
    -> List (Html Message)
viewDropdownItems { screenSize } ({ dropdown } as model) =
    case dropdown of
        Hidden ->
            []

        Shown selectedIdx ->
            let
                dropdownItem : Int -> String -> Html Message
                dropdownItem idx text =
                    Html.li
                        (onMouseDown (FilterMsg text)
                            :: Styles.dropdownItem (Just idx == selectedIdx)
                        )
                        [ Html.text text ]
            in
            [ Html.ul
                (id "search-dropdown" :: Styles.dropdownContainer screenSize)
                (List.indexedMap dropdownItem (dropdownOptions model))
            ]


dropdownOptions :
    { a
        | query : String
        , teams : FetchResult (List Concourse.Team)
        , pipelines : Maybe (Dict String (List Pipeline))
    }
    -> List String
dropdownOptions { query, teams, pipelines } =
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
            Set.union
                (teams
                    |> FetchResult.withDefault []
                    |> List.map .name
                    |> Set.fromList
                )
                (pipelines
                    |> Maybe.withDefault Dict.empty
                    |> Dict.keys
                    |> Set.fromList
                )
                |> Set.toList
                |> List.take 10
                |> List.map (\teamName -> "team: " ++ teamName)

        _ ->
            []
