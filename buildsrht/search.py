from srht.search import search
from buildsrht.types import Job

def apply_search(query, terms):
    if not terms:
        return query
    default_props = [Job.note]
    prop_map = {}
    return search(query, terms, default_props, prop_map)
