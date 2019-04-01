module Build.StepTree.StepTree exposing
    ( extendHighlight
    , finished
    , init
    , setHighlight
    , switchTab
    , toggleStep
    , updateTooltip
    , view
    )

import Ansi.Log
import Array exposing (Array)
import Build.Models exposing (StepHeaderType(..))
import Build.StepTree.Models
    exposing
        ( HookedStep
        , MetadataField
        , Step
        , StepFocus
        , StepName
        , StepState(..)
        , StepTree(..)
        , StepTreeModel
        , TabFocus(..)
        , Version
        , finishTree
        , focusRetry
        , map
        , updateAt
        , wrapHook
        , wrapMultiStep
        , wrapStep
        )
import Build.Styles as Styles
import Concourse
import DateFormat
import Debug
import Dict exposing (Dict)
import Html exposing (Html)
import Html.Attributes exposing (attribute, class, classList, href, style, target)
import Html.Events exposing (onClick, onMouseDown, onMouseEnter, onMouseLeave)
import Message.Effects exposing (Effect(..))
import Message.Message exposing (Hoverable(..), Message(..))
import Routes exposing (Highlight(..), StepID, showHighlight)
import StrictEvents
import Time
import Url exposing (fromString)
import Views.DictView as DictView
import Views.Icon as Icon
import Views.Spinner as Spinner


init :
    Highlight
    -> Concourse.BuildResources
    -> Concourse.BuildPlan
    -> StepTreeModel
init hl resources buildPlan =
    case buildPlan.step of
        Concourse.BuildStepTask name ->
            initBottom hl Task buildPlan.id name

        Concourse.BuildStepGet name version ->
            initBottom hl
                (Get << setupGetStep resources name version)
                buildPlan.id
                name

        Concourse.BuildStepPut name ->
            initBottom hl Put buildPlan.id name

        Concourse.BuildStepAggregate plans ->
            initMultiStep hl resources buildPlan.id Aggregate plans

        Concourse.BuildStepDo plans ->
            initMultiStep hl resources buildPlan.id Do plans

        Concourse.BuildStepRetry plans ->
            initMultiStep hl resources buildPlan.id (Retry buildPlan.id 1 Auto) plans

        Concourse.BuildStepOnSuccess hookedPlan ->
            initHookedStep hl resources OnSuccess hookedPlan

        Concourse.BuildStepOnFailure hookedPlan ->
            initHookedStep hl resources OnFailure hookedPlan

        Concourse.BuildStepOnAbort hookedPlan ->
            initHookedStep hl resources OnAbort hookedPlan

        Concourse.BuildStepEnsure hookedPlan ->
            initHookedStep hl resources Ensure hookedPlan

        Concourse.BuildStepTry plan ->
            initWrappedStep hl resources Try plan

        Concourse.BuildStepTimeout plan ->
            initWrappedStep hl resources Timeout plan


initMultiStep :
    Highlight
    -> Concourse.BuildResources
    -> String
    -> (Array StepTree -> StepTree)
    -> Array Concourse.BuildPlan
    -> StepTreeModel
initMultiStep hl resources planId constructor plans =
    let
        inited =
            Array.map (init hl resources) plans

        trees =
            Array.map .tree inited

        selfFoci =
            Dict.singleton planId { update = identity }

        foci =
            inited
                |> Array.map .foci
                |> Array.indexedMap wrapMultiStep
                |> Array.foldr Dict.union selfFoci
    in
    StepTreeModel (constructor trees) foci hl Nothing


initBottom :
    Highlight
    -> (Step -> StepTree)
    -> StepID
    -> StepName
    -> StepTreeModel
initBottom hl create id name =
    let
        step =
            { id = id
            , name = name
            , state = StepStatePending
            , log = Ansi.Log.init Ansi.Log.Cooked
            , error = Nothing
            , expanded =
                case hl of
                    HighlightNothing ->
                        Nothing

                    HighlightLine stepID _ ->
                        if id == stepID then
                            Just True

                        else
                            Nothing

                    HighlightRange stepID _ _ ->
                        if id == stepID then
                            Just True

                        else
                            Nothing
            , version = Nothing
            , metadata = []
            , firstOccurrence = False
            , timestamps = Dict.empty
            }
    in
    { tree = create step
    , foci = Dict.singleton id { update = identity }
    , highlight = hl
    , tooltip = Nothing
    }


initWrappedStep :
    Highlight
    -> Concourse.BuildResources
    -> (StepTree -> StepTree)
    -> Concourse.BuildPlan
    -> StepTreeModel
initWrappedStep hl resources create plan =
    let
        { tree, foci } =
            init hl resources plan
    in
    { tree = create tree
    , foci = Dict.map wrapStep foci
    , highlight = hl
    , tooltip = Nothing
    }


initHookedStep :
    Highlight
    -> Concourse.BuildResources
    -> (HookedStep -> StepTree)
    -> Concourse.HookedPlan
    -> StepTreeModel
initHookedStep hl resources create hookedPlan =
    let
        stepModel =
            init hl resources hookedPlan.step

        hookModel =
            init hl resources hookedPlan.hook
    in
    { tree = create { step = stepModel.tree, hook = hookModel.tree }
    , foci =
        Dict.union
            (Dict.map wrapStep stepModel.foci)
            (Dict.map wrapHook hookModel.foci)
    , highlight = hl
    , tooltip = Nothing
    }


treeIsActive : StepTree -> Bool
treeIsActive stepTree =
    case stepTree of
        Aggregate trees ->
            List.any treeIsActive (Array.toList trees)

        Do trees ->
            List.any treeIsActive (Array.toList trees)

        OnSuccess { step } ->
            treeIsActive step

        OnFailure { step } ->
            treeIsActive step

        OnAbort { step } ->
            treeIsActive step

        Ensure { step } ->
            treeIsActive step

        Try tree ->
            treeIsActive tree

        Timeout tree ->
            treeIsActive tree

        Retry _ _ _ trees ->
            List.any treeIsActive (Array.toList trees)

        Task step ->
            stepIsActive step

        Get step ->
            stepIsActive step

        Put step ->
            stepIsActive step


stepIsActive : Step -> Bool
stepIsActive =
    isActive << .state


setupGetStep : Concourse.BuildResources -> StepName -> Maybe Version -> Step -> Step
setupGetStep resources name version step =
    { step
        | version = version
        , firstOccurrence = isFirstOccurrence resources.inputs name
    }


isFirstOccurrence : List Concourse.BuildResourcesInput -> StepName -> Bool
isFirstOccurrence resources step =
    case resources of
        [] ->
            False

        { name, firstOccurrence } :: rest ->
            if name == step then
                firstOccurrence

            else
                isFirstOccurrence rest step


finished : StepTreeModel -> StepTreeModel
finished root =
    { root | tree = finishTree root.tree }


toggleStep : StepID -> StepTreeModel -> ( StepTreeModel, List Effect )
toggleStep id root =
    ( updateAt
        id
        (map (\step -> { step | expanded = toggleExpanded step }))
        root
    , []
    )


switchTab : StepID -> Int -> StepTreeModel -> ( StepTreeModel, List Effect )
switchTab id tab root =
    ( updateAt id (focusRetry tab) root, [] )


setHighlight : StepID -> Int -> StepTreeModel -> ( StepTreeModel, List Effect )
setHighlight id line root =
    let
        hl =
            HighlightLine id line
    in
    ( { root | highlight = hl }, [ ModifyUrl (showHighlight hl) ] )


extendHighlight : StepID -> Int -> StepTreeModel -> ( StepTreeModel, List Effect )
extendHighlight id line root =
    let
        hl =
            case root.highlight of
                HighlightNothing ->
                    HighlightLine id line

                HighlightLine currentID currentLine ->
                    if currentID == id then
                        if currentLine < line then
                            HighlightRange id currentLine line

                        else
                            HighlightRange id line currentLine

                    else
                        HighlightLine id line

                HighlightRange currentID currentLine _ ->
                    if currentID == id then
                        if currentLine < line then
                            HighlightRange id currentLine line

                        else
                            HighlightRange id line currentLine

                    else
                        HighlightLine id line
    in
    ( { root | highlight = hl }, [ ModifyUrl (showHighlight hl) ] )


toggleExpanded : Step -> Maybe Bool
toggleExpanded { expanded, state } =
    Just <| not <| Maybe.withDefault (autoExpanded state) expanded


updateTooltip :
    { a
        | hoveredElement : Maybe Hoverable
        , hoveredCounter : Int
    }
    -> StepTreeModel
    -> ( StepTreeModel, List Effect )
updateTooltip { hoveredElement, hoveredCounter } model =
    let
        newTooltip =
            case hoveredElement of
                Just (FirstOccurrenceIcon id) ->
                    if hoveredCounter > 0 then
                        Just id

                    else
                        Nothing

                _ ->
                    Nothing
    in
    ( { model | tooltip = newTooltip }, [] )


view : StepTreeModel -> Html Message
view model =
    viewTree model model.tree


viewTree : StepTreeModel -> StepTree -> Html Message
viewTree model tree =
    case tree of
        Task step ->
            viewStep model step StepHeaderTask

        Get step ->
            viewStep model step (StepHeaderGet step.firstOccurrence)

        Put step ->
            viewStep model step StepHeaderPut

        Try step ->
            viewTree model step

        Retry id tab _ steps ->
            Html.div [ class "retry" ]
                [ Html.ul [ class "retry-tabs" ]
                    (Array.toList <| Array.indexedMap (viewTab id tab) steps)
                , case Array.get (tab - 1) steps of
                    Just step ->
                        viewTree model step

                    Nothing ->
                        Debug.todo "impossible (bogus tab selected)"
                ]

        Timeout step ->
            viewTree model step

        Aggregate steps ->
            Html.div [ class "aggregate" ]
                (Array.toList <| Array.map (viewSeq model) steps)

        Do steps ->
            Html.div [ class "do" ]
                (Array.toList <| Array.map (viewSeq model) steps)

        OnSuccess { step, hook } ->
            viewHooked "success" model step hook

        OnFailure { step, hook } ->
            viewHooked "failure" model step hook

        OnAbort { step, hook } ->
            viewHooked "abort" model step hook

        Ensure { step, hook } ->
            viewHooked "ensure" model step hook


viewTab : StepID -> Int -> Int -> StepTree -> Html Message
viewTab id currentTab idx step =
    let
        tab =
            idx + 1
    in
    Html.li
        [ classList [ ( "current", currentTab == tab ), ( "inactive", not <| treeIsActive step ) ] ]
        [ Html.a [ onClick (SwitchTab id tab) ] [ Html.text (String.fromInt tab) ] ]


viewSeq : StepTreeModel -> StepTree -> Html Message
viewSeq model tree =
    Html.div [ class "seq" ] [ viewTree model tree ]


viewHooked : String -> StepTreeModel -> StepTree -> StepTree -> Html Message
viewHooked name model step hook =
    Html.div [ class "hooked" ]
        [ Html.div [ class "step" ] [ viewTree model step ]
        , Html.div [ class "children" ]
            [ Html.div [ class ("hook hook-" ++ name) ] [ viewTree model hook ]
            ]
        ]


isActive : StepState -> Bool
isActive =
    (/=) StepStatePending


autoExpanded : StepState -> Bool
autoExpanded state =
    isActive state && state /= StepStateSucceeded


viewStep : StepTreeModel -> Step -> StepHeaderType -> Html Message
viewStep model { id, name, log, state, error, expanded, version, metadata, firstOccurrence, timestamps } headerType =
    Html.div
        [ classList
            [ ( "build-step", True )
            , ( "inactive", not <| isActive state )
            ]
        , attribute "data-step-name" name
        ]
        [ Html.div
            ([ class "header"
             , onClick (ToggleStep id)
             ]
                ++ Styles.stepHeader
            )
            [ Html.div
                [ style "display" "flex" ]
                [ viewStepHeaderIcon headerType (model.tooltip == Just id) id
                , Html.h3 [] [ Html.text name ]
                ]
            , Html.div
                [ style "display" "flex" ]
                [ viewVersion version
                , viewStepState state
                ]
            ]
        , Html.div
            [ classList
                [ ( "step-body", True )
                , ( "clearfix", True )
                , ( "step-collapsed", not <| Maybe.withDefault (autoExpanded state) expanded )
                ]
            ]
          <|
            if Maybe.withDefault (autoExpanded state) (Maybe.map (always True) expanded) then
                [ viewMetadata metadata
                , Html.pre [ class "timestamped-logs" ] <|
                    viewLogs log timestamps model.highlight id
                , case error of
                    Nothing ->
                        Html.span [] []

                    Just msg ->
                        Html.span [ class "error" ] [ Html.pre [] [ Html.text msg ] ]
                ]

            else
                []
        ]


viewLogs : Ansi.Log.Model -> Dict Int Time.Posix -> Highlight -> String -> List (Html Message)
viewLogs { lines } timestamps hl id =
    Array.toList <| Array.indexedMap (\idx -> viewTimestampedLine timestamps hl id (idx + 1)) lines


viewTimestampedLine : Dict Int Time.Posix -> Highlight -> StepID -> Int -> Ansi.Log.Line -> Html Message
viewTimestampedLine timestamps hl id lineNo line =
    let
        highlighted =
            case hl of
                HighlightNothing ->
                    False

                HighlightLine hlId hlLine ->
                    hlId == id && hlLine == lineNo

                HighlightRange hlId hlLine1 hlLine2 ->
                    hlId == id && lineNo >= hlLine1 && lineNo <= hlLine2

        ts =
            Dict.get lineNo timestamps
    in
    Html.tr
        [ classList
            [ ( "timestamped-line", True )
            , ( "highlighted-line", highlighted )
            ]
        ]
        [ viewTimestamp hl id ( lineNo, ts )
        , viewLine line
        ]


viewLine : Ansi.Log.Line -> Html Message
viewLine line =
    Html.td [ class "timestamped-content" ]
        [ Ansi.Log.viewLine line
        ]


viewTimestamp : Highlight -> String -> ( Int, Maybe Time.Posix ) -> Html Message
viewTimestamp hl id ( line, date ) =
    Html.a
        [ href (showHighlight (HighlightLine id line))
        , StrictEvents.onLeftClickOrShiftLeftClick
            (SetHighlight id line)
            (ExtendHighlight id line)
        ]
        [ case date of
            Just d ->
                Html.td
                    [ class "timestamp"
                    , attribute "data-timestamp" <|
                        DateFormat.format
                            [ DateFormat.hourMilitaryFixed
                            , DateFormat.text ":"
                            , DateFormat.minuteFixed
                            , DateFormat.text ":"
                            , DateFormat.secondFixed
                            ]
                            Time.utc
                            -- TODO handle timezones
                            d
                    ]
                    []

            _ ->
                Html.td [ class "timestamp placeholder" ] []
        ]


viewVersion : Maybe Version -> Html Message
viewVersion version =
    Maybe.withDefault Dict.empty version
        |> Dict.map (always Html.text)
        |> DictView.view []


viewMetadata : List MetadataField -> Html Message
viewMetadata =
    List.map
        (\{ name, value } ->
            ( name
            , Html.pre []
                [ case fromString value of
                    Just _ ->
                        Html.a
                            [ href value
                            , target "_blank"
                            , style "text-decoration-line" "underline"
                            ]
                            [ Html.text value ]

                    Nothing ->
                        Html.text value
                ]
            )
        )
        >> Dict.fromList
        >> DictView.view []


viewStepState : StepState -> Html Message
viewStepState state =
    case state of
        StepStateRunning ->
            Spinner.spinner { size = "14px", margin = "7px" }

        StepStatePending ->
            Icon.icon
                { sizePx = 28
                , image = "ic-pending.svg"
                }
                ([ attribute "data-step-state" "pending" ]
                    ++ Styles.stepStatusIcon
                )

        StepStateInterrupted ->
            Icon.icon
                { sizePx = 28
                , image = "ic-interrupted.svg"
                }
                ([ attribute "data-step-state" "interrupted" ]
                    ++ Styles.stepStatusIcon
                )

        StepStateCancelled ->
            Icon.icon
                { sizePx = 28
                , image = "ic-cancelled.svg"
                }
                ([ attribute "data-step-state" "cancelled" ]
                    ++ Styles.stepStatusIcon
                )

        StepStateSucceeded ->
            Icon.icon
                { sizePx = 28
                , image = "ic-success-check.svg"
                }
                ([ attribute "data-step-state" "succeeded" ]
                    ++ Styles.stepStatusIcon
                )

        StepStateFailed ->
            Icon.icon
                { sizePx = 28
                , image = "ic-failure-times.svg"
                }
                ([ attribute "data-step-state" "failed" ]
                    ++ Styles.stepStatusIcon
                )

        StepStateErrored ->
            Icon.icon
                { sizePx = 28
                , image = "ic-exclamation-triangle.svg"
                }
                ([ attribute "data-step-state" "errored" ]
                    ++ Styles.stepStatusIcon
                )


viewStepHeaderIcon : StepHeaderType -> Bool -> StepID -> Html Message
viewStepHeaderIcon headerType tooltip id =
    let
        eventHandlers =
            if headerType == StepHeaderGet True then
                [ onMouseLeave <| Hover Nothing
                , onMouseEnter <| Hover <| Just <| FirstOccurrenceIcon id
                ]

            else
                []
    in
    Html.div
        (Styles.stepHeaderIcon headerType ++ eventHandlers)
        (if tooltip then
            [ Html.div
                Styles.firstOccurrenceTooltip
                [ Html.text "new version" ]
            , Html.div
                Styles.firstOccurrenceTooltipArrow
                []
            ]

         else
            []
        )
