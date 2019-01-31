module Build.StepTree exposing
    ( extendHighlight
    , finished
    , init
    , map
    , parseHighlight
    , setHighlight
    , switchTab
    , toggleStep
    , updateAt
    , updateTooltip
    , view
    )

import Ansi.Log
import Array exposing (Array)
import Build.Models
    exposing
        ( Highlight(..)
        , HookedStep
        , Hoverable(..)
        , MetadataField
        , Step
        , StepFocus
        , StepHeaderType(..)
        , StepID
        , StepName
        , StepState(..)
        , StepTree(..)
        , StepTreeModel
        , TabFocus(..)
        , Version
        )
import Build.Msgs exposing (Msg(..))
import Build.Styles as Styles
import Concourse
import Date exposing (Date)
import Date.Format
import Debug
import Dict exposing (Dict)
import DictView
import Effects exposing (Effect(..))
import Focus exposing ((=>), Focus)
import Html exposing (Html)
import Html.Attributes exposing (attribute, class, classList, href, style)
import Html.Events exposing (onClick, onMouseDown, onMouseEnter, onMouseLeave)
import Spinner
import StrictEvents


init :
    Highlight
    -> Concourse.BuildResources
    -> Concourse.BuildPlan
    -> StepTreeModel
init hl resources plan =
    case plan.step of
        Concourse.BuildStepTask name ->
            initBottom hl Task plan.id name

        Concourse.BuildStepGet name version ->
            initBottom hl
                (Get << setupGetStep resources name version)
                plan.id
                name

        Concourse.BuildStepPut name ->
            initBottom hl Put plan.id name

        Concourse.BuildStepAggregate plans ->
            initMultiStep hl resources plan.id Aggregate plans

        Concourse.BuildStepDo plans ->
            initMultiStep hl resources plan.id Do plans

        Concourse.BuildStepRetry plans ->
            initMultiStep hl resources plan.id (Retry plan.id 1 Auto) plans

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
            Dict.singleton planId (Focus.create identity identity)

        foci =
            inited
                |> Array.map .foci
                |> Array.indexedMap wrapMultiStep
                |> Array.foldr Dict.union selfFoci
    in
    StepTreeModel (constructor trees) foci False hl Nothing


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
    , foci = Dict.singleton id (Focus.create identity identity)
    , finished = False
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
    , finished = False
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
    , finished = stepModel.finished
    , highlight = hl
    , tooltip = Nothing
    }


treeIsActive : StepTree -> Bool
treeIsActive tree =
    case tree of
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


toggleStep : StepID -> StepTreeModel -> ( StepTreeModel, List Effect )
toggleStep id root =
    ( updateAt
        id
        (map (\step -> { step | expanded = toggleExpanded step }))
        root
    , []
    )


finished : StepTreeModel -> ( StepTreeModel, List Effect )
finished root =
    ( { root | finished = True }, [] )


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
                Just (FirstOccurrence id) ->
                    if hoveredCounter > 0 then
                        Just id

                    else
                        Nothing

                _ ->
                    Nothing
    in
    ( { model | tooltip = newTooltip }, [] )


focusRetry : Int -> StepTree -> StepTree
focusRetry tab tree =
    case tree of
        Retry id _ _ steps ->
            Retry id tab User steps

        _ ->
            Debug.crash "impossible (non-retry tab focus)"


updateAt : StepID -> (StepTree -> StepTree) -> StepTreeModel -> StepTreeModel
updateAt id update root =
    case Dict.get id root.foci of
        Nothing ->
            Debug.crash ("updateAt: id " ++ id ++ " not found")

        Just focus ->
            { root | tree = Focus.update focus update root.tree }


map : (Step -> Step) -> StepTree -> StepTree
map f tree =
    case tree of
        Task step ->
            Task (f step)

        Get step ->
            Get (f step)

        Put step ->
            Put (f step)

        _ ->
            tree


wrapMultiStep : Int -> Dict StepID StepFocus -> Dict StepID StepFocus
wrapMultiStep i =
    Dict.map (\_ focus -> Focus.create (getMultiStepIndex i) (setMultiStepIndex i) => focus)


wrapStep : StepID -> StepFocus -> StepFocus
wrapStep id subFocus =
    Focus.create getStep updateStep => subFocus


getStep : StepTree -> StepTree
getStep tree =
    case tree of
        OnSuccess { step } ->
            step

        OnFailure { step } ->
            step

        OnAbort { step } ->
            step

        Ensure { step } ->
            step

        Try step ->
            step

        Timeout step ->
            step

        _ ->
            Debug.crash "impossible"


updateStep : (StepTree -> StepTree) -> StepTree -> StepTree
updateStep update tree =
    case tree of
        OnSuccess hookedStep ->
            OnSuccess { hookedStep | step = update hookedStep.step }

        OnFailure hookedStep ->
            OnFailure { hookedStep | step = update hookedStep.step }

        OnAbort hookedStep ->
            OnAbort { hookedStep | step = update hookedStep.step }

        Ensure hookedStep ->
            Ensure { hookedStep | step = update hookedStep.step }

        Try step ->
            Try (update step)

        Timeout step ->
            Timeout (update step)

        _ ->
            Debug.crash "impossible"


wrapHook : StepID -> StepFocus -> StepFocus
wrapHook id subFocus =
    Focus.create getHook updateHook => subFocus


getHook : StepTree -> StepTree
getHook tree =
    case tree of
        OnSuccess { hook } ->
            hook

        OnFailure { hook } ->
            hook

        OnAbort { hook } ->
            hook

        Ensure { hook } ->
            hook

        _ ->
            Debug.crash "impossible"


updateHook : (StepTree -> StepTree) -> StepTree -> StepTree
updateHook update tree =
    case tree of
        OnSuccess hookedStep ->
            OnSuccess { hookedStep | hook = update hookedStep.hook }

        OnFailure hookedStep ->
            OnFailure { hookedStep | hook = update hookedStep.hook }

        OnAbort hookedStep ->
            OnAbort { hookedStep | hook = update hookedStep.hook }

        Ensure hookedStep ->
            Ensure { hookedStep | hook = update hookedStep.hook }

        _ ->
            Debug.crash "impossible"


getMultiStepIndex : Int -> StepTree -> StepTree
getMultiStepIndex idx tree =
    let
        steps =
            case tree of
                Aggregate trees ->
                    trees

                Do trees ->
                    trees

                Retry _ _ _ trees ->
                    trees

                _ ->
                    Debug.crash "impossible"
    in
    case Array.get idx steps of
        Just sub ->
            sub

        Nothing ->
            Debug.crash "impossible"


setMultiStepIndex : Int -> (StepTree -> StepTree) -> StepTree -> StepTree
setMultiStepIndex idx update tree =
    case tree of
        Aggregate trees ->
            Aggregate (Array.set idx (update (getMultiStepIndex idx tree)) trees)

        Do trees ->
            Do (Array.set idx (update (getMultiStepIndex idx tree)) trees)

        Retry id tab focus trees ->
            let
                updatedSteps =
                    Array.set idx (update (getMultiStepIndex idx tree)) trees
            in
            case focus of
                Auto ->
                    Retry id (idx + 1) Auto updatedSteps

                User ->
                    Retry id tab User updatedSteps

        _ ->
            Debug.crash "impossible"


view : StepTreeModel -> Html Msg
view model =
    viewTree model model.tree


viewTree : StepTreeModel -> StepTree -> Html Msg
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
                        Debug.crash "impossible (bogus tab selected)"
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


viewTab : StepID -> Int -> Int -> StepTree -> Html Msg
viewTab id currentTab idx step =
    let
        tab =
            idx + 1
    in
    Html.li
        [ classList [ ( "current", currentTab == tab ), ( "inactive", not <| treeIsActive step ) ] ]
        [ Html.a [ onClick (SwitchTab id tab) ] [ Html.text (toString tab) ] ]


viewSeq : StepTreeModel -> StepTree -> Html Msg
viewSeq model tree =
    Html.div [ class "seq" ] [ viewTree model tree ]


viewHooked : String -> StepTreeModel -> StepTree -> StepTree -> Html Msg
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


viewStep : StepTreeModel -> Step -> StepHeaderType -> Html Msg
viewStep model { id, name, log, state, error, expanded, version, metadata, firstOccurrence, timestamps } headerType =
    Html.div
        [ classList
            [ ( "build-step", True )
            , ( "inactive", not <| isActive state )
            ]
        , attribute "data-step-name" name
        ]
        [ Html.div
            [ class "header"
            , style Styles.stepHeader
            , onClick (ToggleStep id)
            ]
            [ Html.div
                [ style [ ( "display", "flex" ) ] ]
                [ viewStepHeaderIcon headerType (model.tooltip == Just id) id
                , Html.h3 [] [ Html.text name ]
                ]
            , Html.div
                [ style [ ( "display", "flex" ) ] ]
                [ viewVersion version
                , viewStepState state model.finished
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


viewLogs : Ansi.Log.Model -> Dict Int Date -> Highlight -> String -> List (Html Msg)
viewLogs { lines } timestamps hl id =
    Array.toList <| Array.indexedMap (\idx -> viewTimestampedLine timestamps hl id (idx + 1)) lines


viewTimestampedLine : Dict Int Date -> Highlight -> StepID -> Int -> Ansi.Log.Line -> Html Msg
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


viewLine : Ansi.Log.Line -> Html Msg
viewLine line =
    Html.td [ class "timestamped-content" ]
        [ Ansi.Log.viewLine line
        ]


viewTimestamp : Highlight -> String -> ( Int, Maybe Date ) -> Html Msg
viewTimestamp hl id ( line, date ) =
    Html.a
        [ href (showHighlight (HighlightLine id line))
        , StrictEvents.onLeftClickOrShiftLeftClick (SetHighlight id line) (ExtendHighlight id line)
        ]
        [ case date of
            Just date ->
                Html.td [ class "timestamp", attribute "data-timestamp" (Date.Format.format "%H:%M:%S" date) ] []

            _ ->
                Html.td [ class "timestamp placeholder" ] []
        ]


viewVersion : Maybe Version -> Html Msg
viewVersion version =
    Maybe.withDefault Dict.empty version
        |> Dict.map (always Html.text)
        |> DictView.view []


viewMetadata : List MetadataField -> Html Msg
viewMetadata metadata =
    DictView.view []
        << Dict.fromList
    <|
        List.map (\{ name, value } -> ( name, Html.pre [] [ Html.text value ] )) metadata


viewStepState : StepState -> Bool -> Html Msg
viewStepState state buildFinished =
    case state of
        StepStatePending ->
            let
                icon =
                    if buildFinished then
                        "fa-circle"

                    else
                        "fa-circle-o-notch"
            in
            Html.i
                [ class ("right fa fa-fw " ++ icon)
                ]
                []

        StepStateRunning ->
            Spinner.spinner "14px" [ style [ ( "margin", "7px" ) ] ]

        StepStateSucceeded ->
            Html.div
                [ attribute "data-step-state" "succeeded"
                , style <| Styles.stepStatusIcon "ic-success-check"
                ]
                []

        StepStateFailed ->
            Html.div
                [ attribute "data-step-state" "failed"
                , style <| Styles.stepStatusIcon "ic-failure-times"
                ]
                []

        StepStateErrored ->
            Html.div
                [ attribute "data-step-state" "errored"
                , style <| Styles.stepStatusIcon "ic-exclamation-triangle"
                ]
                []


viewStepHeaderIcon : StepHeaderType -> Bool -> StepID -> Html Msg
viewStepHeaderIcon headerType tooltip id =
    let
        eventHandlers =
            if headerType == StepHeaderGet True then
                [ onMouseLeave <| Hover Nothing
                , onMouseEnter <| Hover (Just (FirstOccurrence id))
                ]

            else
                []
    in
    Html.div
        ([ style <| Styles.stepHeaderIcon headerType ] ++ eventHandlers)
        (if tooltip then
            [ Html.div
                [ style Styles.firstOccurrenceTooltip ]
                [ Html.text "new version" ]
            , Html.div
                [ style Styles.firstOccurrenceTooltipArrow ]
                []
            ]

         else
            []
        )


showHighlight : Highlight -> String
showHighlight hl =
    case hl of
        HighlightNothing ->
            ""

        HighlightLine id line ->
            "#L" ++ id ++ ":" ++ toString line

        HighlightRange id line1 line2 ->
            "#L" ++ id ++ ":" ++ toString line1 ++ ":" ++ toString line2


parseHighlight : String -> Highlight
parseHighlight hash =
    case String.uncons (String.dropLeft 1 hash) of
        Just ( 'L', selector ) ->
            case String.split ":" selector of
                [ stepID, line1str, line2str ] ->
                    case ( String.toInt line1str, String.toInt line2str ) of
                        ( Ok line1, Ok line2 ) ->
                            HighlightRange stepID line1 line2

                        _ ->
                            HighlightNothing

                [ stepID, linestr ] ->
                    case String.toInt linestr of
                        Ok line ->
                            HighlightLine stepID line

                        _ ->
                            HighlightNothing

                _ ->
                    HighlightNothing

        _ ->
            HighlightNothing
