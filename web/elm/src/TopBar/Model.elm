module TopBar.Model exposing
    ( Dropdown(..)
    , MiddleSection(..)
    , Model
    , PipelineState
    , isPaused
    , middleSection
    )

import Concourse
import Dashboard.Group.Models exposing (Group)
import Routes
import ScreenSize exposing (ScreenSize(..))


type alias Model r =
    { r
        | isUserMenuExpanded : Bool
        , isPinMenuExpanded : Bool
        , route : Routes.Route
        , groups : List Group
        , dropdown : Dropdown
        , screenSize : ScreenSize
        , shiftDown : Bool
    }



-- The Route in middle section should always be a pipeline, build, resource, or job, but that's hard to demonstrate statically


type MiddleSection
    = Breadcrumbs Routes.Route
    | MinifiedSearch
    | SearchBar
    | Empty


middleSection : Model r -> MiddleSection
middleSection { route, dropdown, screenSize, groups } =
    case route of
        Routes.Dashboard (Routes.Normal query) ->
            let
                q =
                    Maybe.withDefault "" query
            in
            if groups |> List.concatMap .pipelines |> List.isEmpty then
                Empty

            else if dropdown == Hidden && screenSize == Mobile && q == "" then
                MinifiedSearch

            else
                SearchBar

        Routes.Dashboard Routes.HighDensity ->
            Empty

        _ ->
            Breadcrumbs route


type Dropdown
    = Hidden
    | Shown { selectedIdx : Maybe Int }


type alias PipelineState =
    { pinnedResources : List ( String, Concourse.Version )
    , pipeline : Concourse.PipelineIdentifier
    , isPaused : Bool
    , isToggleHovered : Bool
    , isToggleLoading : Bool
    }


isPaused : Maybe PipelineState -> Bool
isPaused =
    Maybe.map .isPaused >> Maybe.withDefault False
