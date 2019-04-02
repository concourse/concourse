module Dashboard.SearchBar exposing
    ( filter
    , handleDelivery
    , searchInputId
    , update
    , view
    )

import Array
import Concourse.PipelineStatus
    exposing
        ( PipelineStatus(..)
        , StatusDetails(..)
        , equal
        , isRunning
        )
import Dashboard.Filter as Filter
import Dashboard.Group.Models exposing (Group, Pipeline)
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
import Simple.Fuzzy


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
            ( { model | dropdown = Shown Nothing }, effects )

        BlurMsg ->
            ( { model | dropdown = Hidden }, effects )

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
                                            (Routes.Normal (Just selectedItem))
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
            [ Html.div
                ([ id "show-search-button"
                 , onClick ShowSearchInput
                 ]
                    ++ Styles.searchButton
                )
                []
            ]

    else
        Html.div
            (id "search-container" :: Styles.searchContainer screenSize)
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


filter : String -> List Group -> List Group
filter query groups =
    let
        filters =
            Filter.filters query

        onlyFilteringTeamNames =
            List.all
                (\f ->
                    case f of
                        Filter.Match (Filter.Team _) ->
                            True

                        _ ->
                            False
                )
                filters

        collapser =
            if onlyFilteringTeamNames then
                identity

            else
                List.filter (.pipelines >> List.isEmpty >> not)
    in
    filters
        |> List.foldr runFilter groups
        |> collapser


runFilter : Filter.Filter -> List Group -> List Group
runFilter f =
    case f of
        Filter.Match (Filter.Team teamName) ->
            List.filter (.teamName >> Simple.Fuzzy.match teamName)

        Filter.Negate (Filter.Team teamName) ->
            List.filter (.teamName >> Simple.Fuzzy.match teamName >> not)

        Filter.Match (Filter.Pipeline pf) ->
            List.map
                (\g ->
                    { g
                        | pipelines =
                            g.pipelines
                                |> List.filter (pipelineFilter pf)
                    }
                )

        Filter.Negate (Filter.Pipeline pf) ->
            List.map
                (\g ->
                    { g
                        | pipelines =
                            g.pipelines
                                |> List.filter (pipelineFilter pf >> not)
                    }
                )


pipelineFilter : Filter.PipelineFilter -> Pipeline -> Bool
pipelineFilter pf =
    case pf of
        Filter.Status sf ->
            case sf of
                Filter.PipelineStatus ps ->
                    .status >> equal ps

                Filter.PipelineRunning ->
                    .status >> isRunning

        Filter.FuzzyName term ->
            .name >> Simple.Fuzzy.match term
