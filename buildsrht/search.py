from buildsrht.types import Job

def apply_search(query, terms):
    if terms:
        # TODO: More advanced search
        for term in terms.split(" "):
            query = query.filter(Job.note.ilike("%" + term + "%"))
    return query
